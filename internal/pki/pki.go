package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config holds PKI paths and parameters.
type Config struct {
	StorageDir string // Base storage dir (e.g. /mnt/allod-storage/security/pki)
}

// Manager orchestrates the Allod Sovereign Family Root CA and 90-day leaf server certificates.
type Manager struct {
	cfg        Config
	mu         sync.RWMutex
	rootCert   *x509.Certificate
	rootKey    *ecdsa.PrivateKey
	serverCert *x509.Certificate
	serverKey  *ecdsa.PrivateKey
	tlsCert    *tls.Certificate
}

// NewManager creates a new PKI manager with the given configuration.
func NewManager(storageDir string) *Manager {
	if storageDir == "" {
		storageDir = "/mnt/allod-storage/security/pki"
	}
	return &Manager{
		cfg: Config{StorageDir: storageDir},
	}
}

// EnsureRootCA ensures that the long-lived (10-year) Sovereign Family Root CA exists on disk,
// generating it if absent.
func (m *Manager) EnsureRootCA() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.cfg.StorageDir, 0755); err != nil {
		return fmt.Errorf("failed to create PKI storage dir %s: %w", m.cfg.StorageDir, err)
	}

	caCertPath := filepath.Join(m.cfg.StorageDir, "allod-root-ca.crt")
	caKeyPath := filepath.Join(m.cfg.StorageDir, "allod-root-ca.key")

	// Try loading existing Root CA
	if certBytes, err := os.ReadFile(caCertPath); err == nil {
		if keyBytes, errK := os.ReadFile(caKeyPath); errK == nil {
			blockCert, _ := pem.Decode(certBytes)
			blockKey, _ := pem.Decode(keyBytes)
			if blockCert != nil && blockKey != nil {
				parsedCert, errP := x509.ParseCertificate(blockCert.Bytes)
				parsedKey, errPK := x509.ParseECPrivateKey(blockKey.Bytes)
				if errP == nil && errPK == nil && time.Now().Before(parsedCert.NotAfter) {
					m.rootCert = parsedCert
					m.rootKey = parsedKey
					return nil
				}
			}
		}
	}

	// Generate new Root CA Key (ECDSA P-256 for optimal security, mobile speed, and battery efficiency)
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Root CA private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate Root CA serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "Allod Sovereign Family Root CA",
			Organization: []string{"Allod Sovereign NAS"},
			OrganizationalUnit: []string{"Security & Privacy"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years validity for family
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		MaxPathLenZero:        false,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("failed to create Root CA certificate: %w", err)
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal Root CA private key: %w", err)
	}
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(caCertPath, caCertPEM, 0644); err != nil {
		return fmt.Errorf("failed to save Root CA certificate: %w", err)
	}
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save Root CA private key: %w", err)
	}

	parsedCert, _ := x509.ParseCertificate(caDER)
	m.rootCert = parsedCert
	m.rootKey = privKey
	log.Printf("[PKI] Generated new Allod Sovereign Family Root CA (valid until %s)", template.NotAfter.Format("2006-01-02"))
	return nil
}

// CollectServerSANs inspects active network interfaces and returns all detected host IPv4/IPv6 addresses
// plus standard local DNS names (localhost, allod.local, and machine hostname).
func CollectServerSANs() (ips []net.IP, dnsNames []string) {
	ipMap := make(map[string]net.IP)
	dnsMap := make(map[string]bool)

	// Standard local DNS
	dnsMap["localhost"] = true
	dnsMap["allod.local"] = true
	dnsMap["*.allod.local"] = true

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		dnsMap[hostname] = true
		dnsMap[hostname+".local"] = true
	}

	// Loopbacks
	ipMap["127.0.0.1"] = net.ParseIP("127.0.0.1")
	ipMap["::1"] = net.ParseIP("::1")

	// Detect all active interface IPs (LAN ethernet/wifi, WireGuard wt0, etc.)
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			if addrs, err := iface.Addrs(); err == nil {
				for _, addr := range addrs {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}
					if ip != nil && !ip.IsLoopback() {
						ipMap[ip.String()] = ip
					}
				}
			}
		}
	}

	// Environment overrides
	if meshIP := os.Getenv("ALLOD_MESH_IP"); meshIP != "" {
		if parsed := net.ParseIP(meshIP); parsed != nil {
			ipMap[meshIP] = parsed
		}
	}

	for _, ip := range ipMap {
		ips = append(ips, ip)
	}
	for d := range dnsMap {
		dnsNames = append(dnsNames, d)
	}
	return ips, dnsNames
}

// EnsureServerCert ensures that a 90-day server certificate exists, is signed by the Root CA,
// covers all current SANs (LAN IP + Mesh IP + allod.local), and has more than 30 days of runway remaining.
// If expiring soon or missing, it automatically regenerates and signs a fresh 90-day certificate.
func (m *Manager) EnsureServerCert() error {
	if err := m.EnsureRootCA(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	srvCertPath := filepath.Join(m.cfg.StorageDir, "server.crt")
	srvKeyPath := filepath.Join(m.cfg.StorageDir, "server.key")

	ips, dnsNames := CollectServerSANs()

	// Check existing server cert validity and SAN coverage
	if certBytes, err := os.ReadFile(srvCertPath); err == nil {
		if keyBytes, errK := os.ReadFile(srvKeyPath); errK == nil {
			blockCert, _ := pem.Decode(certBytes)
			blockKey, _ := pem.Decode(keyBytes)
			if blockCert != nil && blockKey != nil {
				parsedCert, errP := x509.ParseCertificate(blockCert.Bytes)
				parsedKey, errPK := x509.ParseECPrivateKey(blockKey.Bytes)
				// Require at least 25 days before expiration (standard 90-day cycle with auto-renewal at ~65 days)
				if errP == nil && errPK == nil && time.Now().Add(25*24*time.Hour).Before(parsedCert.NotAfter) {
					// Verify that current primary IP is included in SANs
					hasAllSANs := true
					existingIPs := make(map[string]bool)
					for _, ip := range parsedCert.IPAddresses {
						existingIPs[ip.String()] = true
					}
					for _, reqIP := range ips {
						if !existingIPs[reqIP.String()] {
							hasAllSANs = false
							break
						}
					}
					if hasAllSANs {
						m.serverCert = parsedCert
						m.serverKey = parsedKey
						if tlsC, errT := tls.LoadX509KeyPair(srvCertPath, srvKeyPath); errT == nil {
							m.tlsCert = &tlsC
						}
						return nil
					}
				}
			}
		}
	}

	// Generate fresh ECDSA P-256 Key for server certificate
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate server private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate server cert serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "allod.local",
			Organization: []string{"Allod Sovereign NAS"},
		},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(90 * 24 * time.Hour), // 90 days validity (standard modern TLS lifecycle)
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  ips,
		DNSNames:     dnsNames,
	}

	srvDER, err := x509.CreateCertificate(rand.Reader, template, m.rootCert, &privKey.PublicKey, m.rootKey)
	if err != nil {
		return fmt.Errorf("failed to sign server certificate with Root CA: %w", err)
	}

	srvCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srvDER})
	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal server private key: %w", err)
	}
	srvKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(srvCertPath, srvCertPEM, 0644); err != nil {
		return fmt.Errorf("failed to save server certificate: %w", err)
	}
	if err := os.WriteFile(srvKeyPath, srvKeyPEM, 0600); err != nil {
		return fmt.Errorf("failed to save server private key: %w", err)
	}

	parsedCert, _ := x509.ParseCertificate(srvDER)
	m.serverCert = parsedCert
	m.serverKey = privKey
	if tlsC, errT := tls.X509KeyPair(srvCertPEM, srvKeyPEM); errT == nil {
		m.tlsCert = &tlsC
	}

	log.Printf("[PKI] Generated & Signed 90-Day TLS Server Certificate (valid until %s, SANs: %d IPs, %d DNS)",
		template.NotAfter.Format("2006-01-02"), len(ips), len(dnsNames))
	return nil
}

// GetTLSConfig returns a *tls.Config ready for http.Server or listeners, dynamically serving
// the latest rotated certificate.
func (m *Manager) GetTLSConfig() *tls.Config {
	m.mu.RLock()
	var certs []tls.Certificate
	if m.tlsCert != nil {
		certs = []tls.Certificate{*m.tlsCert}
	}
	m.mu.RUnlock()

	return &tls.Config{
		Certificates: certs,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			m.mu.RLock()
			cert := m.tlsCert
			m.mu.RUnlock()
			if cert != nil {
				return cert, nil
			}
			certPath, keyPath := m.GetServerCertPaths()
			c, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, err
			}
			m.mu.Lock()
			m.tlsCert = &c
			m.mu.Unlock()
			return &c, nil
		},
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
	}
}

// GetRootCAFingerprintSHA256 returns the formatted SHA-256 fingerprint of the Root CA.
func (m *Manager) GetRootCAFingerprintSHA256() (string, error) {
	m.mu.RLock()
	if m.rootCert != nil {
		raw := m.rootCert.Raw
		m.mu.RUnlock()
		sum := sha256.Sum256(raw)
		var hexParts []string
		for _, b := range sum {
			hexParts = append(hexParts, fmt.Sprintf("%02X", b))
		}
		return strings.Join(hexParts, ":"), nil
	}
	m.mu.RUnlock()

	if err := m.EnsureRootCA(); err != nil {
		return "", err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.rootCert == nil {
		return "", fmt.Errorf("root certificate not available")
	}
	sum := sha256.Sum256(m.rootCert.Raw)
	var hexParts []string
	for _, b := range sum {
		hexParts = append(hexParts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(hexParts, ":"), nil
}

// GetStatus returns metadata regarding current Root CA and active Leaf Certificate.
func (m *Manager) GetStatus() map[string]interface{} {
	_ = m.EnsureServerCert()
	fingerprint, _ := m.GetRootCAFingerprintSHA256()

	m.mu.RLock()
	defer m.mu.RUnlock()

	var caCN, caValidUntil string
	if m.rootCert != nil {
		caCN = m.rootCert.Subject.CommonName
		caValidUntil = m.rootCert.NotAfter.Format(time.RFC3339)
	}

	var srvValidUntil string
	var daysRemaining int
	var sans []string
	if m.serverCert != nil {
		srvValidUntil = m.serverCert.NotAfter.Format(time.RFC3339)
		daysRemaining = int(time.Until(m.serverCert.NotAfter).Hours() / 24)
		for _, ip := range m.serverCert.IPAddresses {
			sans = append(sans, ip.String())
		}
		sans = append(sans, m.serverCert.DNSNames...)
	}

	return map[string]interface{}{
		"ca_common_name":     caCN,
		"ca_valid_until":     caValidUntil,
		"ca_fingerprint":     fingerprint,
		"server_valid_until":  srvValidUntil,
		"days_remaining":     daysRemaining,
		"sans":               sans,
		"storage_dir":        m.cfg.StorageDir,
		"auto_renew":         true,
	}
}

// GetRootCACertPEM returns the PEM-encoded Root CA certificate ready for client installation.
func (m *Manager) GetRootCACertPEM() ([]byte, error) {
	if err := m.EnsureRootCA(); err != nil {
		return nil, err
	}
	caCertPath := filepath.Join(m.cfg.StorageDir, "allod-root-ca.crt")
	return os.ReadFile(caCertPath)
}

// GetServerCertPaths returns the absolute paths to the current server.crt and server.key files on disk.
func (m *Manager) GetServerCertPaths() (certPath string, keyPath string) {
	return filepath.Join(m.cfg.StorageDir, "server.crt"), filepath.Join(m.cfg.StorageDir, "server.key")
}

// GenerateAppleMobileConfig generates an Apple `.mobileconfig` XML profile payload containing
// the Root CA certificate. On iOS / iPadOS and macOS, tapping this profile installs the CA in 1 click.
func (m *Manager) GenerateAppleMobileConfig() ([]byte, error) {
	rootPEM, err := m.GetRootCACertPEM()
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(rootPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode root CA PEM")
	}

	// Base64 DER payload for Apple Profile
	certBase64 := strings.ReplaceAll(string(pem.EncodeToMemory(block)), "-----BEGIN CERTIFICATE-----", "")
	certBase64 = strings.ReplaceAll(certBase64, "-----END CERTIFICATE-----", "")
	certBase64 = strings.TrimSpace(certBase64)

	profileUUID := "64BA49BA-26B5-4134-9DBC-07B94A3E3B07"
	payloadUUID := "A110D000-0000-4000-8000-000000000001"

	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadCertificateFileName</key>
			<string>AllodFamilyRootCA.crt</string>
			<key>PayloadContent</key>
			<data>
%s
			</data>
			<key>PayloadDescription</key>
			<string>Allod Sovereign Family Root Certificate Authority for local LAN and Sovereign Mesh HTTPS.</string>
			<key>PayloadDisplayName</key>
			<string>Allod Sovereign Family Root CA</string>
			<key>PayloadIdentifier</key>
			<string>dev.allod.security.rootca</string>
			<key>PayloadType</key>
			<string>com.apple.security.root</string>
			<key>PayloadUUID</key>
			<string>%s</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDescription</key>
	<string>Installa il certificato di sicurezza radice della tua famiglia per proteggere tutte le password, foto e file personali su Allod con crittografia TLS 1.3.</string>
	<key>PayloadDisplayName</key>
	<string>Allod Family Security Profile (HTTPS)</string>
	<key>PayloadIdentifier</key>
	<string>dev.allod.profile.family-root</string>
	<key>PayloadOrganization</key>
	<string>Allod Sovereign NAS</string>
	<key>PayloadRemovalDisallowed</key>
	<false/>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>%s</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>
`, certBase64, payloadUUID, profileUUID)

	return []byte(xml), nil
}

// StartAutoRenewal initiates a background worker that checks the server certificate every 12 hours
// and transparently renews it whenever less than 25 days of validity remain.
func (m *Manager) StartAutoRenewal() {
	go func() {
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := m.EnsureServerCert(); err != nil {
				log.Printf("[PKI-AUTO-RENEW] Error checking/renewing server certificate: %v", err)
			}
		}
	}()
}
