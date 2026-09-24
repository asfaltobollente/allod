package wol

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"
)

// MagicPacket creates the standard AMD Magic Packet (102 bytes).
// It consists of 6 bytes of 0xFF followed by 16 repetitions of the target MAC address.
func MagicPacket(mac net.HardwareAddr) ([]byte, error) {
	if len(mac) != 6 {
		return nil, fmt.Errorf("invalid MAC address length: expected 6 bytes, got %d", len(mac))
	}

	var packet bytes.Buffer
	// 6 bytes of 0xFF
	packet.Write(bytes.Repeat([]byte{0xFF}, 6))
	// 16 repetitions of the 6-byte MAC
	for i := 0; i < 16; i++ {
		packet.Write(mac)
	}

	return packet.Bytes(), nil
}

// ParseMAC normalizes and parses a MAC address string in various formats
// (colon-separated, hyphen-separated, or unseparated hex).
func ParseMAC(macStr string) (net.HardwareAddr, error) {
	cleaned := strings.TrimSpace(macStr)
	// Try standard parsing first
	hw, err := net.ParseMAC(cleaned)
	if err == nil && len(hw) == 6 {
		return hw, nil
	}

	// Handle unseparated hex strings (e.g. 00D861330E1F)
	hexOnly := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			return r
		}
		return -1
	}, cleaned)

	if len(hexOnly) == 12 {
		var formatted strings.Builder
		for i := 0; i < 12; i += 2 {
			if i > 0 {
				formatted.WriteByte(':')
			}
			formatted.WriteString(hexOnly[i : i+2])
		}
		hw, err = net.ParseMAC(formatted.String())
		if err == nil && len(hw) == 6 {
			return hw, nil
		}
	}

	return nil, fmt.Errorf("invalid MAC address %q: must be 6 hex pairs (e.g. 00:11:22:33:44:55)", macStr)
}

// WakeResult holds diagnostic details regarding the sent Wake-on-LAN packets.
type WakeResult struct {
	MACAddress   string    `json:"mac_address"`
	BroadcastIP  string    `json:"broadcast_ip"`
	Port         int       `json:"port"`
	Interfaces   []string  `json:"interfaces"`
	PacketsSent  int       `json:"packets_sent"`
	SentAt       time.Time `json:"sent_at"`
}

// Send broadcasts a Wake-on-LAN magic packet for the specified MAC address.
// To guarantee delivery on multi-homed hosts (e.g. hosts with WireGuard/NetBird interfaces),
// it sends to the general broadcast address (255.255.255.255) as well as each active
// physical network interface's computed subnet broadcast address.
func Send(macStr string, broadcastIP string, port int) (*WakeResult, error) {
	mac, err := ParseMAC(macStr)
	if err != nil {
		return nil, err
	}

	packet, err := MagicPacket(mac)
	if err != nil {
		return nil, err
	}

	if port <= 0 {
		port = 9
	}
	if strings.TrimSpace(broadcastIP) == "" {
		broadcastIP = "255.255.255.255"
	}

	result := &WakeResult{
		MACAddress:  mac.String(),
		BroadcastIP: broadcastIP,
		Port:        port,
		Interfaces:  make([]string, 0),
		SentAt:      time.Now(),
	}

	// 1. Send to specified target broadcast IP
	targetAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", broadcastIP, port))
	if err == nil {
		if conn, err := net.DialUDP("udp4", nil, targetAddr); err == nil {
			_, _ = conn.Write(packet)
			_ = conn.Close()
			result.PacketsSent++
			result.Interfaces = append(result.Interfaces, fmt.Sprintf("global (%s)", broadcastIP))
		}
	}

	// 2. Discover local physical/LAN network interfaces and broadcast on each subnet
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			// Skip loopback and down interfaces
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			// Only broadcast on interfaces that support broadcast
			if iface.Flags&net.FlagBroadcast == 0 {
				continue
			}

			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok || ipNet.IP.IsLoopback() {
					continue
				}

				ipv4 := ipNet.IP.To4()
				if ipv4 == nil {
					continue
				}

				// Compute subnet broadcast: IP | ^Mask
				mask := ipNet.Mask
				if len(mask) != 4 {
					continue
				}

				bcast := net.IPv4(
					ipv4[0]|^mask[0],
					ipv4[1]|^mask[1],
					ipv4[2]|^mask[2],
					ipv4[3]|^mask[3],
				)

				// Bind socket to this interface's local IP and send to subnet broadcast
				localAddr := &net.UDPAddr{IP: ipv4, Port: 0}
				remoteAddr := &net.UDPAddr{IP: bcast, Port: port}

				conn, err := net.DialUDP("udp4", localAddr, remoteAddr)
				if err == nil {
					_, _ = conn.Write(packet)
					_ = conn.Close()
					result.PacketsSent++
					result.Interfaces = append(result.Interfaces, fmt.Sprintf("%s (%s -> %s)", iface.Name, ipv4, bcast))
				}
			}
		}
	}

	if result.PacketsSent == 0 {
		return nil, fmt.Errorf("failed to send magic packet: no broadcast route available")
	}

	return result, nil
}
