// Package security provides HTTP security middleware for the API service.
package security

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Headers adds security headers (HSTS, CSP, X-Frame-Options, etc.) to every response.
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// HSTS — 1 year, include subdomains
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		// Block rendering in frames
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		// CSP: API only serves JSON + Redoc docs; allow the CDN for Redoc
		h.Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self' cdn.redoc.ly; "+
				"style-src 'self' 'unsafe-inline' fonts.googleapis.com; "+
				"font-src fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self'; "+
				"frame-ancestors 'none'",
		)
		next.ServeHTTP(w, r)
	})
}

// privateRanges contains all IANA-reserved / private IPv4 and IPv6 ranges.
var privateRanges = func() []*net.IPNet {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",  // link-local
		"100.64.0.0/10",   // shared address space
		"::1/128",         // IPv6 loopback
		"fc00::/7",        // IPv6 unique local
		"fe80::/10",       // IPv6 link-local
		"0.0.0.0/8",       // "this" network
		"192.0.2.0/24",    // TEST-NET-1
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved
		"255.255.255.255/32",
	}
	var nets []*net.IPNet
	for _, c := range cidrs {
		_, n, _ := net.ParseCIDR(c)
		if n != nil {
			nets = append(nets, n)
		}
	}
	return nets
}()

// IsPrivateIP reports whether ip falls in any IANA-reserved range.
func IsPrivateIP(ip net.IP) bool {
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateEndpointURL checks that rawURL is a safe, public HTTPS endpoint.
// Returns a non-nil error string if the URL should be rejected.
func ValidateEndpointURL(rawURL string) string {
	if rawURL == "" {
		return "endpoint URL is required"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "invalid URL"
	}
	if !strings.EqualFold(u.Scheme, "https") && !strings.EqualFold(u.Scheme, "http") {
		return "only http/https URLs are allowed"
	}
	host := u.Hostname()
	// Block bare IPs that fall in private ranges
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivateIP(ip) {
			return "endpoint URL resolves to a private IP address"
		}
		return ""
	}
	// Block localhost / metadata endpoint hostnames
	lower := strings.ToLower(host)
	blocked := []string{"localhost", "metadata.google.internal", "169.254.169.254"}
	for _, b := range blocked {
		if lower == b || strings.HasSuffix(lower, "."+b) {
			return "endpoint URL hostname is not allowed"
		}
	}
	return ""
}
