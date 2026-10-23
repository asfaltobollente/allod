package gateway

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/asfaltobollente/allod/internal/pki"
)

// Route defines the mapping between an external TLS port and an internal module container port.
type Route struct {
	ModuleID     string
	ExternalPort int
	InternalPort int
	Title        string
	Description  string
}

// DefaultRoutes lists the default web-based module mappings.
var DefaultRoutes = []Route{
	{
		ModuleID:     "passwords",
		ExternalPort: 8088,
		InternalPort: 18088,
		Title:        "Allod Passwords (Vaultwarden)",
		Description:  "Gestore password e credenziali crittografato end-to-end.",
	},
	{
		ModuleID:     "photos",
		ExternalPort: 2283,
		InternalPort: 12283,
		Title:        "Allod Photos (Immich)",
		Description:  "Backup e galleria fotografica familiare ad alta risoluzione.",
	},
	{
		ModuleID:     "cloud",
		ExternalPort: 8443,
		InternalPort: 18443,
		Title:        "Allod Cloud (Nextcloud)",
		Description:  "Sincronizzazione file, calendari e contatti sovrana.",
	},
	{
		ModuleID:     "media",
		ExternalPort: 8096,
		InternalPort: 18096,
		Title:        "Allod Media (Jellyfin)",
		Description:  "Streaming multimediale domestico.",
	},
	{
		ModuleID:     "search",
		ExternalPort: 8888,
		InternalPort: 18888,
		Title:        "Allod Search (SearXNG)",
		Description:  "Metamotore di ricerca privato senza tracciamento.",
	},
	{
		ModuleID:     "odysseus",
		ExternalPort: 7000,
		InternalPort: 17000,
		Title:        "Allod Odysseus (AI Assistant)",
		Description:  "Assistente IA e spazio di lavoro cognitivo locale.",
	},
}

// Gateway manages all TLS listeners and reverse proxies for Allod modules.
type Gateway struct {
	pkiMgr    *pki.Manager
	routes    []Route
	mu        sync.Mutex
	servers   map[int]*http.Server
	listeners map[int]net.Listener
	stopCh    chan struct{}
}

// New creates a new TLS Gateway instance using certificates provided by pkiMgr.
func New(pkiMgr *pki.Manager) *Gateway {
	return &Gateway{
		pkiMgr:    pkiMgr,
		routes:    DefaultRoutes,
		servers:   make(map[int]*http.Server),
		listeners: make(map[int]net.Listener),
		stopCh:    make(chan struct{}),
	}
}

// Start launches TLS reverse proxies for all defined routes.
func (g *Gateway) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	tlsConfig := g.pkiMgr.GetTLSConfig()

	for _, route := range g.routes {
		r := route // copy for closure
		addr := fmt.Sprintf(":%d", r.ExternalPort)

		rawListener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("[GATEWAY] ⚠️ Impossibile ascoltare su porta %d: %v (porta già occupata?)", r.ExternalPort, err)
			continue
		}

		smartListener := &sniffListener{
			underlying: rawListener,
			tlsConfig:  tlsConfig,
			extPort:    r.ExternalPort,
		}

		proxy := g.buildProxy(r)
		server := &http.Server{
			Handler:      proxy,
			TLSConfig:    tlsConfig,
			ReadTimeout:  120 * time.Second,
			WriteTimeout: 120 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		g.listeners[r.ExternalPort] = smartListener
		g.servers[r.ExternalPort] = server

		go func(p int, srv *http.Server, l net.Listener, mod string) {
			log.Printf("[GATEWAY] 🛡️ TLS Gateway attivo su porta %d -> 127.0.0.1:%d (Modulo: %s)", p, r.InternalPort, mod)
			if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
				log.Printf("[GATEWAY] Listener porta %d terminato: %v", p, err)
			}
		}(r.ExternalPort, server, smartListener, r.ModuleID)
	}

	return nil
}

// Stop gracefully stops all TLS Gateway listeners and connections.
func (g *Gateway) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for port, srv := range g.servers {
		_ = srv.Close()
		delete(g.servers, port)
	}
	for port, l := range g.listeners {
		_ = l.Close()
		delete(g.listeners, port)
	}
}

func (g *Gateway) buildProxy(route Route) http.Handler {
	targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", route.InternalPort))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Set("X-Forwarded-Proto", "https")
		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			req.Header.Set("X-Forwarded-For", clientIP)
			req.Header.Set("X-Real-IP", clientIP)
		}
		req.Header.Set("X-Forwarded-Host", req.Host)
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		renderModuleOfflinePage(w, route)
	}

	return proxy
}

func renderModuleOfflinePage(w http.ResponseWriter, r Route) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="it">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="refresh" content="4">
<title>%s - In Avvio</title>
<style>
  :root {
    --bg: #090a0f;
    --card: #12151e;
    --border: rgba(255, 255, 255, 0.08);
    --primary: #3b82f6;
    --text: #f3f4f6;
    --muted: #9ca3af;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    background-color: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: 20px;
  }
  .card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 16px;
    padding: 32px;
    max-width: 480px;
    width: 100%%;
    text-align: center;
    box-shadow: 0 20px 40px rgba(0,0,0,0.4);
  }
  .spinner {
    width: 48px;
    height: 48px;
    border: 3px solid rgba(59, 130, 246, 0.2);
    border-top-color: var(--primary);
    border-radius: 50%%;
    margin: 0 auto 24px;
    animation: spin 1s linear infinite;
  }
  @keyframes spin { to { transform: rotate(360deg); } }
  h1 { font-size: 20px; font-weight: 600; margin-bottom: 8px; }
  .badge {
    display: inline-block;
    padding: 4px 10px;
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
    border-radius: 20px;
    font-size: 12px;
    font-weight: 600;
    margin-bottom: 16px;
  }
  p { color: var(--muted); font-size: 14px; line-height: 1.5; margin-bottom: 24px; }
  .btn {
    display: inline-block;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid var(--border);
    color: var(--text);
    text-decoration: none;
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 13px;
    font-weight: 500;
    transition: background 0.15s;
  }
  .btn:hover { background: rgba(255, 255, 255, 0.1); }
</style>
</head>
<body>
<div class="card">
  <div class="spinner"></div>
  <div class="badge">Crittografia TLS 1.3 Attiva</div>
  <h1>%s</h1>
  <p>%s<br><br>Il modulo è attualmente in fase di avvio o temporaneamente non attivo su questo nodo. La pagina si aggiornerà automaticamente tra pochi istanti.</p>
  <a class="btn" href="javascript:location.reload()">Riprova Adesso</a>
</div>
</body>
</html>`, r.Title, r.Title, r.Description)

	_, _ = w.Write([]byte(html))
}

// sniffListener inspects the first byte of incoming TCP connections:
// - If 0x16 (TLS ClientHello): wraps conn with tls.Server
// - If NOT 0x16 (Plain HTTP, e.g. 'G', 'P'): immediately responds with HTTP 307 redirect to HTTPS
type sniffListener struct {
	underlying net.Listener
	tlsConfig  *tls.Config
	extPort    int
}

func (l *sniffListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.underlying.Accept()
		if err != nil {
			return nil, err
		}

		// Set short deadline to peek the first byte
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var firstByte [1]byte
		n, err := conn.Read(firstByte[:])
		_ = conn.SetReadDeadline(time.Time{})

		if err != nil || n == 0 {
			_ = conn.Close()
			continue
		}

		if firstByte[0] == 0x16 {
			// TLS Handshake ClientHello
			replayConn := &prefixConn{
				Conn:   conn,
				prefix: firstByte[:],
			}
			return tls.Server(replayConn, l.tlsConfig), nil
		}

		// Plain HTTP connection: read request line & host header to redirect to HTTPS
		go l.handleHTTPRedirect(conn, firstByte[0])
	}
}

func (l *sniffListener) handleHTTPRedirect(conn net.Conn, firstByte byte) {
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	prefixReader := io.MultiReader(bytes.NewReader([]byte{firstByte}), conn)
	bufReader := bufio.NewReader(prefixReader)

	reqLine, err := bufReader.ReadString('\n')
	if err != nil {
		return
	}

	parts := strings.Split(strings.TrimSpace(reqLine), " ")
	reqURI := "/"
	if len(parts) >= 2 {
		reqURI = parts[1]
	}

	host := ""
	for {
		line, err := bufReader.ReadString('\n')
		if err != nil || line == "\r\n" || line == "\n" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "host:") {
			host = strings.TrimSpace(line[5:])
		}
	}

	if host == "" {
		host = conn.LocalAddr().String()
	}

	// Clean port if mismatched
	if strings.Contains(host, ":") {
		h, _, errH := net.SplitHostPort(host)
		if errH == nil {
			host = fmt.Sprintf("%s:%d", h, l.extPort)
		}
	} else {
		host = fmt.Sprintf("%s:%d", host, l.extPort)
	}

	redirectURL := fmt.Sprintf("https://%s%s", host, reqURI)
	resp := fmt.Sprintf("HTTP/1.1 307 Temporary Redirect\r\nLocation: %s\r\nConnection: close\r\nContent-Length: 0\r\n\r\n", redirectURL)
	_, _ = conn.Write([]byte(resp))
}

func (l *sniffListener) Close() error {
	return l.underlying.Close()
}

func (l *sniffListener) Addr() net.Addr {
	return l.underlying.Addr()
}

type prefixConn struct {
	net.Conn
	prefix []byte
}

func (c *prefixConn) Read(b []byte) (int, error) {
	if len(c.prefix) > 0 {
		n := copy(b, c.prefix)
		c.prefix = c.prefix[n:]
		return n, nil
	}
	return c.Conn.Read(b)
}
