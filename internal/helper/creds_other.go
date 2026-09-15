//go:build !linux

package helper

import "net"

type defaultPeerCredChecker struct{}

func (c defaultPeerCredChecker) CheckPeer(conn net.Conn) (uint32, bool, error) {
	// Fallback for non-Linux development/testing
	return 0, true, nil
}

func defaultPeerCredCheckerInstance() PeerCredChecker {
	return defaultPeerCredChecker{}
}
