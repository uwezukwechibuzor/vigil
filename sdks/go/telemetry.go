// Package vigil — anonymous opt-out telemetry for the Go SDK.
//
// What is sent: surface, version, OS, runtime, method, success/fail, latency bucket.
// What is NEVER sent: wallet addresses, private keys, calldata, API keys, paths, emails.
// Opt out: VIGIL_TELEMETRY=0 or DO_NOT_TRACK=1
package vigil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	telEndpoint      = "https://app.posthog.com/capture/"
	telPHKey         = "phc_vigil_sdk_anonymous"
	telBatchInterval = 60 * time.Second
	telSendTimeout   = 3 * time.Second
	telSDKVersion    = "0.1.0"
)

var (
	ethAddrRE = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	hex32RE   = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
	piiKeys   = map[string]bool{
		"email": true, "password": true, "private_key": true,
		"mnemonic": true, "seed": true, "address": true,
		"from": true, "to": true, "data": true, "calldata": true,
		"tx": true, "key": true, "secret": true,
	}
)

type telEvent struct {
	event string
	props map[string]any
}

type telClient struct {
	mu    sync.Mutex
	queue []telEvent
	timer *time.Timer
	once  sync.Once
}

var globalTel = &telClient{}

// TelIsEnabled reports whether telemetry is currently on.
func TelIsEnabled() bool {
	if os.Getenv("VIGIL_TELEMETRY") == "0" {
		return false
	}
	if os.Getenv("DO_NOT_TRACK") == "1" {
		return false
	}
	return true
}

// telTrack queues an event for async batch delivery.
func telTrack(event string, props map[string]any) {
	if !TelIsEnabled() {
		return
	}
	globalTel.once.Do(func() {
		_, _ = os.Stderr.WriteString("[vigil] Anonymous telemetry is enabled. Disable with VIGIL_TELEMETRY=0.\n")
	})
	globalTel.mu.Lock()
	defer globalTel.mu.Unlock()
	globalTel.queue = append(globalTel.queue, telEvent{event: event, props: sanitizeTel(props)})
	if globalTel.timer == nil {
		globalTel.timer = time.AfterFunc(telBatchInterval, func() {
			globalTel.mu.Lock()
			batch := globalTel.queue
			globalTel.queue = nil
			globalTel.timer = nil
			globalTel.mu.Unlock()
			sendTelBatch(batch)
		})
	}
}

// TelFlush forces immediate delivery (used in tests).
func TelFlush() {
	globalTel.mu.Lock()
	if globalTel.timer != nil {
		globalTel.timer.Stop()
		globalTel.timer = nil
	}
	batch := globalTel.queue
	globalTel.queue = nil
	globalTel.mu.Unlock()
	sendTelBatch(batch)
}

func sanitizeTel(props map[string]any) map[string]any {
	out := make(map[string]any, len(props))
	for k, v := range props {
		if piiKeys[strings.ToLower(k)] {
			continue
		}
		if s, ok := v.(string); ok {
			if ethAddrRE.MatchString(s) || hex32RE.MatchString(s) {
				continue
			}
		}
		out[k] = v
	}
	return out
}

func sendTelBatch(batch []telEvent) {
	if len(batch) == 0 {
		return
	}
	type event struct {
		Event      string         `json:"event"`
		Properties map[string]any `json:"properties"`
	}
	phBatch := make([]event, 0, len(batch))
	for _, e := range batch {
		p := make(map[string]any, len(e.props)+6)
		for k, v := range e.props {
			p[k] = v
		}
		p["$lib"] = "vigil-sdk-go"
		p["$lib_version"] = telSDKVersion
		p["surface"] = "sdk_go"
		p["distinct_id"] = telAnonID()
		p["go_version"] = runtime.Version()
		p["os_family"] = runtime.GOOS
		phBatch = append(phBatch, event{Event: e.event, Properties: p})
	}
	payload := map[string]any{"api_key": telPHKey, "batch": phBatch}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), telSendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func telAnonID() string {
	return "sdk-go-" + runtime.GOOS + "-" + runtime.Version()
}
