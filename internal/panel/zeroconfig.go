package panel

import (
	"fmt"
	"net"
	"strings"

	"github.com/asfaltobollente/allod/internal/state"
)

// ZeroConfigMode represents a connectivity profile for SMB shares and web services.
type ZeroConfigMode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Badge       string `json:"badge,omitempty"`
	Host        string `json:"host"`
	WinPath     string `json:"win_path"`
	MacPath     string `json:"mac_path"`
	ImmichURL   string `json:"immich_url"`
	JellyfinURL string `json:"jellyfin_url"`
	Description string `json:"description"`
	Recommended bool   `json:"recommended"`
}

// GetPrimaryLANIP returns the main local IPv4 address of the host.
// It prioritizes outbound routing discovery via UDP connect, falling back
// to scanning physical, non-virtual network interfaces.
func GetPrimaryLANIP() string {
	// 1. Preferred method: socket routing probe (no packets are transmitted)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && localAddr != nil {
			ip := localAddr.IP.To4()
			if ip != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}

	// 2. Fallback: inspect physical network interfaces
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			// Skip down, loopback, or virtual/container/mesh interfaces
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			name := strings.ToLower(iface.Name)
			if strings.HasPrefix(name, "wt") || // NetBird
				strings.HasPrefix(name, "docker") ||
				strings.HasPrefix(name, "podman") ||
				strings.HasPrefix(name, "veth") ||
				strings.HasPrefix(name, "br-") ||
				strings.HasPrefix(name, "cni") {
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
				if ipv4 != nil {
					return ipv4.String()
				}
			}
		}
	}

	return "127.0.0.1"
}

// GetNetBirdIP returns the IPv4 address assigned to the wt0 NetBird interface, if active.
func GetNetBirdIP() string {
	iface, err := net.InterfaceByName("wt0")
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return ""
}

// GetZeroConfigHostname retrieves the configured mDNS name (default: "allod").
func GetZeroConfigHostname(st *state.Store) string {
	if st != nil {
		if custom, err := st.GetMeta("mdns_hostname"); err == nil && custom != "" {
			return strings.TrimSpace(custom)
		}
	}
	return "allod"
}

// BuildZeroConfigModes builds structured connectivity options for clients.
func BuildZeroConfigModes(username, lanIP, mdnsHost, netbirdIP string) map[string]ZeroConfigMode {
	if mdnsHost == "" {
		mdnsHost = "allod.local"
	}
	if lanIP == "" {
		lanIP = "127.0.0.1"
	}

	modes := map[string]ZeroConfigMode{
		"mdns": {
			ID:          "mdns",
			Label:       fmt.Sprintf("%s (Zero-Config)", mdnsHost),
			Badge:       "Consigliato",
			Host:        mdnsHost,
			WinPath:     fmt.Sprintf(`\\%s\%s`, mdnsHost, username),
			MacPath:     fmt.Sprintf("smb://%s/%s", mdnsHost, username),
			ImmichURL:   fmt.Sprintf("http://%s:2283", mdnsHost),
			JellyfinURL: fmt.Sprintf("http://%s:8096", mdnsHost),
			Description: "Risoluzione automatica mDNS: funziona su Windows 10/11, macOS, iOS e Android senza configurare il modem/router.",
			Recommended: true,
		},
		"lan_ip": {
			ID:          "lan_ip",
			Label:       fmt.Sprintf("%s (IP Diretto)", lanIP),
			Badge:       "Universale",
			Host:        lanIP,
			WinPath:     fmt.Sprintf(`\\%s\%s`, lanIP, username),
			MacPath:     fmt.Sprintf("smb://%s/%s", lanIP, username),
			ImmichURL:   fmt.Sprintf("http://%s:2283", lanIP),
			JellyfinURL: fmt.Sprintf("http://%s:8096", lanIP),
			Description: "Compatibilità 100% con qualsiasi dispositivo in rete locale. Con la rotta NetBird LAN, funziona anche fuori casa!",
			Recommended: false,
		},
	}

	if netbirdIP != "" {
		modes["mesh"] = ZeroConfigMode{
			ID:          "mesh",
			Label:       fmt.Sprintf("%s (Mesh Remoto)", netbirdIP),
			Badge:       "NetBird VPN",
			Host:        netbirdIP,
			WinPath:     fmt.Sprintf(`\\%s\%s`, netbirdIP, username),
			MacPath:     fmt.Sprintf("smb://%s/%s", netbirdIP, username),
			ImmichURL:   fmt.Sprintf("http://%s:2283", netbirdIP),
			JellyfinURL: fmt.Sprintf("http://%s:8096", netbirdIP),
			Description: "Accesso sicuro da qualsiasi luogo tramite tunnel WireGuard crittografato NetBird.",
			Recommended: false,
		}
	}

	return modes
}

// GenerateAvahiHosts generates the content for /etc/avahi/hosts to publish allod.local.
func GenerateAvahiHosts(lanIP, hostname string) string {
	cleanHost := strings.TrimSuffix(hostname, ".local")
	return fmt.Sprintf("# Generated by Allod Zero-Config\n%s %s.local %s\n", lanIP, cleanHost, cleanHost)
}

// GenerateAvahiService generates the XML service definition for Avahi DNS-SD.
// It announces Samba file sharing and gives Apple macOS Finder a native NAS server icon.
func GenerateAvahiService(serviceName string, smbPort, httpPort int) string {
	if serviceName == "" {
		serviceName = "Allod NAS"
	}
	if smbPort <= 0 {
		smbPort = 445
	}
	if httpPort <= 0 {
		httpPort = 8080
	}

	return fmt.Sprintf(`<?xml version="1.0" standalone='no'?>
<!DOCTYPE service-group SYSTEM "avahi-service.dtd">
<service-group>
  <name replace-wildcards="yes">%s</name>
  <service>
    <type>_smb._tcp</type>
    <port>%d</port>
  </service>
  <service>
    <type>_device-info._tcp</type>
    <port>0</port>
    <txt-record>model=RackMac</txt-record>
  </service>
  <service>
    <type>_http._tcp</type>
    <port>%d</port>
    <txt-record>path=/</txt-record>
  </service>
</service-group>
`, serviceName, smbPort, httpPort)
}

// GenerateWsddDefaultConfig generates the configuration string for /etc/default/wsdd.
func GenerateWsddDefaultConfig(netbiosName, workgroup string) string {
	if netbiosName == "" {
		netbiosName = "ALLOD"
	}
	if workgroup == "" {
		workgroup = "WORKGROUP"
	}
	return fmt.Sprintf(`# /etc/default/wsdd - Allod WS-Discovery for Windows 10/11
WSDD_PARAMS="-n %s -w %s"
`, strings.ToUpper(netbiosName), strings.ToUpper(workgroup))
}
