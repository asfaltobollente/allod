package helper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExecuteSnapshotCreate handles the snapshots.create action, validating targets
// and issuing a real read-only Btrfs snapshot command.
func ExecuteSnapshotCreate(storageRoot, subvol, name string, readonly bool, isPlan bool) Response {
	if storageRoot == "" {
		storageRoot = getStorageBaseDir()
	}

	if subvol == "" {
		subvol = "shares"
	}
	if !validNameRegex.MatchString(subvol) {
		return Response{Ok: false, Error: "Invalid 'subvolume' name"}
	}

	if name == "" {
		name = fmt.Sprintf("%s-%s", subvol, time.Now().Format("20060102-150405"))
	}
	if !validNameRegex.MatchString(name) {
		return Response{Ok: false, Error: "Invalid snapshot 'name'"}
	}

	srcPath := filepath.ToSlash(filepath.Join(storageRoot, subvol))
	if !isAllowedPath(srcPath) {
		return Response{Ok: false, Error: "Source subvolume path is outside allowed storage roots: " + srcPath}
	}

	snapshotDir := filepath.ToSlash(filepath.Join(storageRoot, ".snapshots"))
	targetPath := filepath.ToSlash(filepath.Join(snapshotDir, name))

	flag := ""
	if readonly {
		flag = "-r "
	}
	plan := []string{fmt.Sprintf("btrfs subvolume snapshot %s%s %s", flag, srcPath, targetPath)}

	if isPlan {
		return Response{Ok: true, Applied: false, Plan: plan}
	}

	// Execution phase: ensure src exists
	if fi, err := os.Stat(srcPath); err != nil || !fi.IsDir() {
		return Response{Ok: false, Error: fmt.Sprintf("Source subvolume path '%s' does not exist or is not a directory", srcPath)}
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(snapshotDir, 0750); err != nil {
		return Response{Ok: false, Error: fmt.Sprintf("Failed to create snapshot directory '%s': %v", snapshotDir, err)}
	}

	btrfsBin := resolveExecutable("btrfs", "/bin/btrfs", "/usr/bin/btrfs", "/sbin/btrfs")

	args := []string{"subvolume", "snapshot"}
	if readonly {
		args = append(args, "-r")
	}
	args = append(args, srcPath, targetPath)

	out, err := exec.Command(btrfsBin, args...).CombinedOutput()
	if err != nil {
		return Response{
			Ok:    false,
			Error: fmt.Sprintf("btrfs snapshot failed: %v (%s)", err, strings.TrimSpace(string(out))),
		}
	}

	return Response{
		Ok:      true,
		Applied: true,
		Output:  fmt.Sprintf("Snapshot created: %s", targetPath),
		Plan:    plan,
	}
}

// snapshotEntry captures filesystem entry and modification timestamp for pruning.
type snapshotEntry struct {
	Name    string
	Path    string
	ModTime time.Time
}

// ExecuteSnapshotPrune prunes Btrfs snapshots based on retention count and max age.
func ExecuteSnapshotPrune(storageRoot, subvol string, keep int, maxAgeHours int, isPlan bool) Response {
	if storageRoot == "" {
		storageRoot = getStorageBaseDir()
	}

	if subvol != "" && !validNameRegex.MatchString(subvol) {
		return Response{Ok: false, Error: "Invalid 'subvolume' filter"}
	}

	if keep <= 0 {
		keep = 7 // Default: retain 7 most recent snapshots
	}

	snapshotDir := filepath.ToSlash(filepath.Join(storageRoot, ".snapshots"))
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return Response{Ok: true, Applied: !isPlan, Output: "No snapshots directory found"}
		}
		return Response{Ok: false, Error: fmt.Sprintf("Failed to read snapshot directory: %v", err)}
	}

	var matched []snapshotEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if subvol != "" {
			if !strings.HasPrefix(name, subvol+"-") && name != subvol {
				continue
			}
		}
		info, errI := e.Info()
		modTime := time.Now()
		if errI == nil {
			modTime = info.ModTime()
		}
		matched = append(matched, snapshotEntry{
			Name:    name,
			Path:    filepath.ToSlash(filepath.Join(snapshotDir, name)),
			ModTime: modTime,
		})
	}

	// Sort newest first
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].ModTime.After(matched[j].ModTime)
	})

	var toDelete []snapshotEntry
	now := time.Now()

	for idx, s := range matched {
		deleteThis := false
		if idx >= keep {
			deleteThis = true
		} else if maxAgeHours > 0 {
			age := now.Sub(s.ModTime)
			if age > time.Duration(maxAgeHours)*time.Hour {
				deleteThis = true
			}
		}
		if deleteThis {
			toDelete = append(toDelete, s)
		}
	}

	var plan []string
	for _, d := range toDelete {
		plan = append(plan, fmt.Sprintf("btrfs subvolume delete %s", d.Path))
	}
	if len(plan) == 0 {
		plan = []string{"No snapshots meet pruning criteria"}
	}

	if isPlan {
		return Response{Ok: true, Applied: false, Plan: plan}
	}

	btrfsBin := resolveExecutable("btrfs", "/bin/btrfs", "/usr/bin/btrfs", "/sbin/btrfs")
	deletedCount := 0

	for _, d := range toDelete {
		out, err := exec.Command(btrfsBin, "subvolume", "delete", d.Path).CombinedOutput()
		if err != nil {
			// Fallback if directory isn't recognized as btrfs subvolume
			_ = os.RemoveAll(d.Path)
		}
		deletedCount++
		_ = out
	}

	return Response{
		Ok:      true,
		Applied: true,
		Output:  fmt.Sprintf("Pruned %d snapshot(s)", deletedCount),
		Plan:    plan,
	}
}
