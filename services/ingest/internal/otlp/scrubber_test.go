package otlp

import (
	"strings"
	"testing"
)

func TestScrubString_PrivateKey(t *testing.T) {
	privKey := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	result := ScrubString(privKey)
	if !strings.Contains(result, "[REDACTED_PRIVKEY]") {
		t.Errorf("expected private key to be redacted, got: %s", result)
	}
}

func TestScrubString_PrivateKey0x(t *testing.T) {
	privKey := "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	result := ScrubString(privKey)
	if !strings.Contains(result, "[REDACTED_PRIVKEY]") {
		t.Errorf("expected 0x-prefixed private key to be redacted, got: %s", result)
	}
}

func TestScrubString_Mnemonic12(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	result := ScrubString(mnemonic)
	if !strings.Contains(result, "[REDACTED_MNEMONIC]") {
		t.Errorf("expected mnemonic to be redacted, got: %s", result)
	}
}

func TestScrubString_Mnemonic24(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon " +
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art"
	result := ScrubString(mnemonic)
	if !strings.Contains(result, "[REDACTED_MNEMONIC]") {
		t.Errorf("expected 24-word mnemonic to be redacted, got: %s", result)
	}
}

func TestScrubString_APIKey(t *testing.T) {
	apiKey := "sk-abcdefghijklmnopqrstuvwxyz1234567890"
	result := ScrubString(apiKey)
	if !strings.Contains(result, "[REDACTED_APIKEY]") {
		t.Errorf("expected API key to be redacted, got: %s", result)
	}
}

func TestScrubString_NormalAddress(t *testing.T) {
	// A 40-hex-char Ethereum address should NOT be redacted.
	addr := "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	result := ScrubString(addr)
	if strings.Contains(result, "[REDACTED_PRIVKEY]") {
		t.Errorf("expected normal address NOT to be redacted, got: %s", result)
	}
	if result != addr {
		t.Errorf("address should pass through unchanged, got: %s", result)
	}
}

func TestScrubString_NormalText(t *testing.T) {
	text := "swap 0.1 ETH to USDC on base"
	result := ScrubString(text)
	if result != text {
		t.Errorf("normal text should pass through unchanged, got: %s", result)
	}
}

func TestScrubString_InlinePrivKey(t *testing.T) {
	privKey := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	text := "key=" + privKey + " other=value"
	result := ScrubString(text)
	if strings.Contains(result, privKey) {
		t.Errorf("expected inline private key to be redacted, got: %s", result)
	}
}
