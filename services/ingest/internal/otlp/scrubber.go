// Package otlp handles OTLP/HTTP ingestion, parsing, and PII scrubbing.
package otlp

import (
	"regexp"
	"strings"
	"unicode"

	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

var (
	// rePrivKeyFull matches a full 0x-prefixed 64-hex-char private key (66 chars total).
	rePrivKeyFull = regexp.MustCompile(`(?i)0x[0-9a-fA-F]{64}`)
	// reAPIKey matches common API key patterns such as sk-… or Bearer …
	reAPIKey = regexp.MustCompile(`(?i)(sk-|Bearer )[A-Za-z0-9._\-]{20,}`)
)

// ScrubString redacts secrets from a free-form string. Exported for unit testing.
func ScrubString(s string) string {
	// Redact API keys first.
	s = reAPIKey.ReplaceAllString(s, "[REDACTED_APIKEY]")

	// Redact mnemonic seed phrases: 12 or 24 lowercase words ≥3 chars.
	s = redactMnemonic(s)

	// Redact 0x-prefixed 64-hex private keys first (these are longer than addresses).
	s = rePrivKeyFull.ReplaceAllString(s, "[REDACTED_PRIVKEY]")

	// Redact bare 64-hex private keys that are not part of longer hex strings.
	// We need to be careful not to redact Ethereum addresses (0x + 40 hex chars).
	s = redactBarePrivKey(s)

	return s
}

// redactBarePrivKey replaces standalone 64-hex strings (not Ethereum addresses).
// Ethereum addresses are 40 hex chars; a 64-hex string is a private key.
func redactBarePrivKey(s string) string {
	// Match runs of hex characters.
	reHexRun := regexp.MustCompile(`(?i)[0-9a-fA-F]+`)
	return reHexRun.ReplaceAllStringFunc(s, func(run string) string {
		// Only replace exact 64-char hex strings.
		if len(run) == 64 {
			return "[REDACTED_PRIVKEY]"
		}
		return run
	})
}

// redactMnemonic replaces sequences of 12 or 24 lowercase BIP-39 style words with a placeholder.
func redactMnemonic(s string) string {
	words := strings.Fields(s)
	if len(words) < 12 {
		return s
	}
	for _, count := range []int{24, 12} {
		if len(words) < count {
			continue
		}
		for start := 0; start <= len(words)-count; start++ {
			slice := words[start : start+count]
			if isMnemonicSlice(slice) {
				phrase := strings.Join(slice, " ")
				s = strings.Replace(s, phrase, "[REDACTED_MNEMONIC]", 1)
				// Re-split after replacement since string changed.
				words = strings.Fields(s)
				break
			}
		}
	}
	return s
}

// isMnemonicSlice returns true if all words are lowercase alpha with length ≥ 3.
func isMnemonicSlice(words []string) bool {
	for _, w := range words {
		if len(w) < 3 {
			return false
		}
		for _, r := range w {
			if !unicode.IsLower(r) || !unicode.IsLetter(r) {
				return false
			}
		}
	}
	return true
}

// Scrub redacts PII and secrets from a span's attributes in-place.
// It scrubs all Attrs map values and VigilIntent.
// If VigilWallet looks like a 64-hex private key (not an address), it is redacted.
func Scrub(span *dbch.Span) {
	for k, v := range span.Attrs {
		span.Attrs[k] = ScrubString(v)
	}
	span.VigilIntent = ScrubString(span.VigilIntent)

	// If VigilWallet is 64 hex chars (a private key, not a 40-char address), scrub it.
	w := strings.TrimPrefix(span.VigilWallet, "0x")
	w = strings.TrimPrefix(w, "0X")
	if len(w) == 64 && isHex(w) {
		span.VigilWallet = "[REDACTED_PRIVKEY]"
	}
}

// isHex returns true if all characters in s are hexadecimal digits.
func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
