//go:build !linux

package helper

// RepairSystemBinPermissions is a no-op on non-Linux platforms.
func RepairSystemBinPermissions() error {
	return nil
}
