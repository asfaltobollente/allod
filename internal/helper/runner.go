package helper

import (
	"fmt"
)

// CommandRunner defines the high-level, type-safe interface for privileged operations
// executed via the root helper daemon.
type CommandRunner interface {
	CreateUser(username string) error
	DeleteUser(username string) error
	SetSambaPassword(username, password string) error
	AddSambaUser(username string) error
	DeleteSambaUser(username string) error
	ApplyShare(name, path string) error
	BindPhotos(username string, enabled bool) error
	RestartService(unit string) error
	InitStorage(mode string, disks []string, mountPoint, user string) error
	StorageDiagnostics(mountPoint string) (string, error)
	SnapshotCreate(subvol, name string) error
}

var _ CommandRunner = (*Client)(nil)

// CreateUser provisions a rootless system user with an unprivileged nologin shell.
func (c *Client) CreateUser(username string) error {
	res, err := c.Execute("users.create", map[string]interface{}{"username": username}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error creating user %s: %s", username, res.Error)
	}
	return nil
}

// DeleteUser removes an existing Linux system user.
func (c *Client) DeleteUser(username string) error {
	res, err := c.Execute("users.delete", map[string]interface{}{"username": username}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error deleting user %s: %s", username, res.Error)
	}
	return nil
}

// SetSambaPassword sets the SMB password for a user via smbpasswd.
func (c *Client) SetSambaPassword(username, password string) error {
	res, err := c.Execute("shares.set_password", map[string]interface{}{
		"username": username,
		"password": password,
	}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error setting samba password for %s: %s", username, res.Error)
	}
	return nil
}

// AddSambaUser registers a user with the Samba password database.
func (c *Client) AddSambaUser(username string) error {
	res, err := c.Execute("shares.user_add", map[string]interface{}{"username": username}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error adding samba user %s: %s", username, res.Error)
	}
	return nil
}

// DeleteSambaUser removes a user from the Samba password database.
func (c *Client) DeleteSambaUser(username string) error {
	res, err := c.Execute("shares.user_del", map[string]interface{}{"username": username}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error deleting samba user %s: %s", username, res.Error)
	}
	return nil
}

// ApplyShare configures permissions and directories for a shared path.
func (c *Client) ApplyShare(name, path string) error {
	res, err := c.Execute("shares.apply", map[string]interface{}{
		"name": name,
		"path": path,
	}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error applying share %s: %s", name, res.Error)
	}
	return nil
}

// BindPhotos configures or unlinks the Immich library bind-mount into the user's Samba share.
func (c *Client) BindPhotos(username string, enabled bool) error {
	res, err := c.Execute("shares.bind_photos", map[string]interface{}{
		"username": username,
		"enabled":  enabled,
	}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error binding photos for %s: %s", username, res.Error)
	}
	return nil
}

// RestartService requests the helper to restart a privileged systemd unit.
func (c *Client) RestartService(unit string) error {
	res, err := c.Execute("service.restart", map[string]interface{}{"unit": unit}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error restarting %s: %s", unit, res.Error)
	}
	return nil
}

// InitStorage formats physical drives and creates the Btrfs pool on the specified mount point.
func (c *Client) InitStorage(mode string, disks []string, mountPoint, user string) error {
	res, err := c.Execute("storage.init", map[string]interface{}{
		"mode":  mode,
		"disks": disks,
		"mount": mountPoint,
		"user":  user,
	}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error initializing storage: %s", res.Error)
	}
	return nil
}

// StorageDiagnostics gathers Btrfs and SMART diagnostic status from the helper.
func (c *Client) StorageDiagnostics(mountPoint string) (string, error) {
	res, err := c.Execute("storage.diagnostics", map[string]interface{}{"mount": mountPoint}, false)
	if err != nil {
		return "", fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return "", fmt.Errorf("helper error retrieving storage diagnostics: %s", res.Error)
	}
	return res.Output, nil
}

// SnapshotCreate takes an instant read-only Btrfs snapshot of a subvolume.
func (c *Client) SnapshotCreate(subvol, name string) error {
	res, err := c.Execute("snapshot.create", map[string]interface{}{
		"subvol": subvol,
		"name":   name,
	}, false)
	if err != nil {
		return fmt.Errorf("helper connection error: %w", err)
	}
	if !res.Ok {
		return fmt.Errorf("helper error creating snapshot: %s", res.Error)
	}
	return nil
}
