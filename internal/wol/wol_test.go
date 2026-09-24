package wol

import (
	"bytes"
	"net"
	"testing"
)

func TestParseMAC(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"00:D8:61:33:0E:1F", "00:d8:61:33:0e:1f", true},
		{"00-D8-61-33-0E-1F", "00:d8:61:33:0e:1f", true},
		{"00d861330e1f", "00:d8:61:33:0e:1f", true},
		{"  00:11:22:33:44:55  ", "00:11:22:33:44:55", true},
		{"invalid-mac", "", false},
		{"00:11:22:33:44", "", false},
		{"00:11:22:33:44:55:66", "", false},
	}

	for _, c := range cases {
		hw, err := ParseMAC(c.input)
		if c.valid {
			if err != nil {
				t.Errorf("expected valid for %q, got error: %v", c.input, err)
			} else if hw.String() != c.expected {
				t.Errorf("for input %q expected %q, got %q", c.input, c.expected, hw.String())
			}
		} else {
			if err == nil {
				t.Errorf("expected error for invalid input %q, got none (hw: %s)", c.input, hw)
			}
		}
	}
}

func TestMagicPacketFormat(t *testing.T) {
	mac, err := ParseMAC("00:D8:61:33:0E:1F")
	if err != nil {
		t.Fatalf("failed to parse MAC: %v", err)
	}

	packet, err := MagicPacket(mac)
	if err != nil {
		t.Fatalf("failed to generate magic packet: %v", err)
	}

	// 1. Length must be exactly 102 bytes (6 + 16*6)
	if len(packet) != 102 {
		t.Fatalf("expected 102 bytes, got %d", len(packet))
	}

	// 2. First 6 bytes must be 0xFF
	expectedPrefix := bytes.Repeat([]byte{0xFF}, 6)
	if !bytes.Equal(packet[:6], expectedPrefix) {
		t.Errorf("first 6 bytes mismatch: got %x, expected %x", packet[:6], expectedPrefix)
	}

	// 3. Next 16 blocks must match the 6-byte MAC
	for i := 0; i < 16; i++ {
		offset := 6 + i*6
		chunk := packet[offset : offset+6]
		if !bytes.Equal(chunk, mac) {
			t.Errorf("repetition %d mismatch: got %x, expected %x", i, chunk, mac)
		}
	}
}

func TestSendToMockListener(t *testing.T) {
	// Start a local UDP listener to verify actual transmission
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start UDP listener: %v", err)
	}
	defer conn.Close()

	port := conn.LocalAddr().(*net.UDPAddr).Port

	// Send to 127.0.0.1 on the listener port
	res, err := Send("00:D8:61:33:0E:1F", "127.0.0.1", port)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if res.PacketsSent < 1 {
		t.Errorf("expected at least 1 packet sent, got %d", res.PacketsSent)
	}

	// Read packet from listener
	buf := make([]byte, 256)
	n, _, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatalf("failed to read from UDP listener: %v", err)
	}

	if n != 102 {
		t.Errorf("expected 102 bytes read, got %d", n)
	}
}
