package security_test

import (
	"net"
	"testing"

	"github.com/uwezukwechibuzor/vigil/services/api/internal/security"
)

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"172.16.5.5", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"169.254.169.254", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		got := security.IsPrivateIP(ip)
		if got != c.private {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", c.ip, got, c.private)
		}
	}
}

func TestValidateEndpointURL(t *testing.T) {
	cases := []struct {
		url     string
		wantErr bool
	}{
		{"https://mainnet.infura.io/v3/abc", false},
		{"http://base-mainnet.example.com/rpc", false},
		{"https://10.0.0.1/rpc", true},           // private IP
		{"https://127.0.0.1:8545", true},         // localhost
		{"https://localhost/rpc", true},          // localhost hostname
		{"https://169.254.169.254/latest", true}, // AWS metadata
		{"ftp://example.com", true},              // wrong scheme
		{"", true},                               // empty
	}
	for _, c := range cases {
		errMsg := security.ValidateEndpointURL(c.url)
		if (errMsg != "") != c.wantErr {
			t.Errorf("ValidateEndpointURL(%q) errMsg=%q wantErr=%v", c.url, errMsg, c.wantErr)
		}
	}
}
