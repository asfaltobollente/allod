package panel

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// CheckSameOrigin verifies that mutating HTTP requests (POST, PUT, DELETE, PATCH)
// have an Origin or Referer header matching the request Host.
// Returns true if the request is safe (read-only or valid same-origin mutating request).
func CheckSameOrigin(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}

	expectedHost := r.Host
	if expectedHost == "" {
		return false
	}

	// 1. Check Origin header
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return false
		}
		return isHostMatch(u.Host, expectedHost)
	}

	// 2. Check Referer header if Origin is absent
	if referer := r.Header.Get("Referer"); referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.Host == "" {
			return false
		}
		return isHostMatch(u.Host, expectedHost)
	}

	// Neither Origin nor Referer provided on mutating request
	return false
}

func isHostMatch(sourceHost, targetHost string) bool {
	if strings.EqualFold(sourceHost, targetHost) {
		return true
	}

	srcH, srcP, err1 := net.SplitHostPort(sourceHost)
	tgtH, tgtP, err2 := net.SplitHostPort(targetHost)

	// If both have explicit ports and they differ, reject
	if err1 == nil && err2 == nil {
		if srcP != tgtP {
			return false
		}
		return strings.EqualFold(srcH, tgtH)
	}

	// If source has port and target doesn't (or vice versa), only match if port is standard 80 or 443
	if err1 == nil && err2 != nil {
		if srcP == "80" || srcP == "443" {
			return strings.EqualFold(srcH, targetHost)
		}
		return false
	}
	if err1 != nil && err2 == nil {
		if tgtP == "80" || tgtP == "443" {
			return strings.EqualFold(sourceHost, tgtH)
		}
		return false
	}

	return strings.EqualFold(sourceHost, targetHost)
}
