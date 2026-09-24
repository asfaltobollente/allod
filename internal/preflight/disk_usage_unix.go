//go:build !windows

package preflight

import "golang.org/x/sys/unix"

// GetDiskUsage returns total and used bytes for the specified filesystem mount path.
func GetDiskUsage(path string) (totalBytes int64, usedBytes int64, err error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		// Fallback to root if path is not yet mounted
		if errRoot := unix.Statfs("/", &stat); errRoot == nil {
			total := int64(stat.Blocks) * int64(stat.Bsize)
			free := int64(stat.Bavail) * int64(stat.Bsize)
			return total, total - free, nil
		}
		return 0, 0, err
	}
	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)
	return total, total - free, nil
}
