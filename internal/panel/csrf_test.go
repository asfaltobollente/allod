package panel

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckSameOrigin(t *testing.T) {
	// 1. GET requests are safe and always allowed
	reqGet := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	reqGet.Host = "127.0.0.1:8080"
	if !CheckSameOrigin(reqGet) {
		t.Errorf("expected GET request to be allowed without Origin")
	}

	// 2. Mutating POST without Origin or Referer -> rejected
	reqNoOrigin := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqNoOrigin.Host = "127.0.0.1:8080"
	if CheckSameOrigin(reqNoOrigin) {
		t.Errorf("SECURITY: expected mutating POST without Origin/Referer to be rejected")
	}

	// 3. Mutating POST with matching Origin -> allowed
	reqValidOrigin := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqValidOrigin.Host = "127.0.0.1:8080"
	reqValidOrigin.Header.Set("Origin", "http://127.0.0.1:8080")
	if !CheckSameOrigin(reqValidOrigin) {
		t.Errorf("expected POST with matching Origin to be allowed")
	}

	// 4. Mutating POST with cross-origin Origin (attacker domain) -> rejected
	reqEvilOrigin := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqEvilOrigin.Host = "127.0.0.1:8080"
	reqEvilOrigin.Header.Set("Origin", "http://attacker.example.com")
	if CheckSameOrigin(reqEvilOrigin) {
		t.Errorf("SECURITY: expected cross-origin POST from attacker.example.com to be rejected")
	}

	// 5. Mutating POST from another service on same IP but different port (e.g. Immich on 2283 or Jellyfin on 8096) -> rejected
	reqSameIPDiffPort := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqSameIPDiffPort.Host = "127.0.0.1:8080"
	reqSameIPDiffPort.Header.Set("Origin", "http://127.0.0.1:8096")
	if CheckSameOrigin(reqSameIPDiffPort) {
		t.Errorf("SECURITY: expected POST from different port on same IP to be rejected")
	}

	// 6. Mutating POST with matching Referer (when Origin is omitted by browser) -> allowed
	reqValidReferer := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqValidReferer.Host = "allod.lan:8080"
	reqValidReferer.Header.Set("Referer", "http://allod.lan:8080/settings")
	if !CheckSameOrigin(reqValidReferer) {
		t.Errorf("expected POST with matching Referer to be allowed")
	}

	// 7. Mutating POST with mismatched Referer -> rejected
	reqEvilReferer := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqEvilReferer.Host = "allod.lan:8080"
	reqEvilReferer.Header.Set("Referer", "http://evil.example.com/exploit")
	if CheckSameOrigin(reqEvilReferer) {
		t.Errorf("SECURITY: expected POST with mismatched Referer to be rejected")
	}

	// 8. Standard HTTP port 80 equivalence (host without port vs host with :80)
	reqStdPort := httptest.NewRequest(http.MethodPost, "/api/modules/set", nil)
	reqStdPort.Host = "allod.lan"
	reqStdPort.Header.Set("Origin", "http://allod.lan:80")
	if !CheckSameOrigin(reqStdPort) {
		t.Errorf("expected POST with standard port 80 to match host without port")
	}
}
