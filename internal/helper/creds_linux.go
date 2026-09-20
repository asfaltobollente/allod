//go:build linux

package helper

import (
	"fmt"
	"net"
	"os/user"
	"strconv"

	"golang.org/x/sys/unix"
)

type defaultPeerCredChecker struct{}

func (c defaultPeerCredChecker) CheckPeer(conn net.Conn) (uint32, bool, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, false, fmt.Errorf("not a unix connection")
	}

	rawConn, err := unixConn.SyscallConn()
	if err != nil {
		return 0, false, err
	}

	var ucred *unix.Ucred
	var sysErr error
	err = rawConn.Control(func(fd uintptr) {
		ucred, sysErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return 0, false, err
	}
	if sysErr != nil {
		return 0, false, sysErr
	}

	uid := ucred.Uid
	// Root is always authorized
	if uid == 0 {
		return uid, true, nil
	}

	// Check if caller belongs to 'allod' group
	u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10))
	if err != nil {
		return uid, false, fmt.Errorf("failed to lookup uid %d: %w", uid, err)
	}

	gids, err := u.GroupIds()
	if err != nil {
		return uid, false, fmt.Errorf("failed to lookup group ids for user %s: %w", u.Username, err)
	}

	adminGroups := []string{"allod", "sudo", "wheel", "admin"}
	for _, grpName := range adminGroups {
		if grp, err := user.LookupGroup(grpName); err == nil {
			for _, gid := range gids {
				if gid == grp.Gid {
					return uid, true, nil
				}
			}
		}
	}

	return uid, false, nil
}

func defaultPeerCredCheckerInstance() PeerCredChecker {
	return defaultPeerCredChecker{}
}
