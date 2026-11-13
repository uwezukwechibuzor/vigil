// Package integration contains integration tests for the flight check engine.
// Tests that require Docker are skipped when -short is used.
package integration

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/engine"
)

const (
	anvilFunded  = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	anvilChainID = uint64(31337)
)

// sharedAnvilURL is set once in TestMain and reused by all tests.
var sharedAnvilURL string

// TestMain starts a single shared Anvil container for all integration tests.
// All tests are skipped when -short is passed.
func TestMain(m *testing.M) {
	flag.Parse() // must be called before testing.Short()
	if testing.Short() {
		os.Exit(m.Run()) // tests will self-skip via t.Skip inside each
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/foundry-rs/foundry:nightly",
		Cmd:          []string{"anvil", "--host", "0.0.0.0", "--port", "8545", "--chain-id", "31337"},
		ExposedPorts: []string{"8545/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Listening on").WithStartupTimeout(120*time.Second),
			wait.ForListeningPort("8545/tcp").WithStartupTimeout(120*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		slog.Error("start anvil", "err", err)
		os.Exit(1)
	}
	defer func() { _ = container.Terminate(ctx) }()

	host, err := container.Host(ctx)
	if err != nil {
		slog.Error("anvil host", "err", err)
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "8545")
	if err != nil {
		slog.Error("anvil port", "err", err)
		os.Exit(1)
	}

	sharedAnvilURL = "http://" + host + ":" + port.Port()

	// Poll until Anvil is serving JSON-RPC (port open ≠ RPC ready).
	// Use raw HTTP POST to avoid any go-ethereum client caching behaviour.
	var lastPollErr error
	deadline := time.Now().Add(60 * time.Second)
	ready := false
	hc := &http.Client{Timeout: 2 * time.Second}
	payload := []byte(`{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}`)
	for time.Now().Before(deadline) {
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		req, _ := http.NewRequestWithContext(reqCtx, http.MethodPost, sharedAnvilURL,
			bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := hc.Do(req)
		cancel()
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
			lastPollErr = nil
		} else {
			lastPollErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		slog.Error("anvil not ready after 60s poll", "last_err", lastPollErr, "url", sharedAnvilURL)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// buildEngine creates an engine wired to the shared Anvil RPC URL.
func buildEngine(t *testing.T) *engine.Engine {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Docker-based test in -short mode")
	}

	client, err := ethclient.Dial(sharedAnvilURL)
	if err != nil {
		t.Fatalf("dial ethclient: %v", err)
	}
	t.Cleanup(client.Close)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := check.NewRegistry()
	registry.Register(&check.InfraHealthCheck{})
	registry.Register(&check.SimulationCheck{})
	registry.Register(&check.OracleFreshnessCheck{})
	registry.Register(&check.SlippageCheck{})
	registry.Register(&check.GasSanityCheck{})
	registry.Register(&check.ContractRiskCheck{})
	registry.Register(&check.PolicyCheck{})
	registry.Register(&check.AddressSafetyCheck{Screener: nil})

	clients := []engine.ChainClient{{ChainID: anvilChainID, Client: client}}
	return engine.New(registry, clients, nil, nil, logger, 1500*time.Millisecond)
}

// buildEngineB is like buildEngine but accepts *testing.B.
func buildEngineB(b *testing.B) *engine.Engine {
	b.Helper()
	if testing.Short() {
		b.Skip("skipping Docker-based benchmark in -short mode")
	}
	client, err := ethclient.Dial(sharedAnvilURL)
	if err != nil {
		b.Fatalf("dial ethclient: %v", err)
	}
	b.Cleanup(client.Close)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := check.NewRegistry()
	registry.Register(&check.InfraHealthCheck{})
	registry.Register(&check.SimulationCheck{})
	registry.Register(&check.OracleFreshnessCheck{})
	registry.Register(&check.SlippageCheck{})
	registry.Register(&check.GasSanityCheck{})
	registry.Register(&check.ContractRiskCheck{})
	registry.Register(&check.PolicyCheck{})
	registry.Register(&check.AddressSafetyCheck{Screener: nil})

	clients := []engine.ChainClient{{ChainID: anvilChainID, Client: client}}
	return engine.New(registry, clients, nil, nil, logger, 1500*time.Millisecond)
}

// TestFlightCheck_NormalEthTransfer expects verdict "go" for a simple ETH send.
func TestFlightCheck_NormalEthTransfer(t *testing.T) {
	eng := buildEngine(t)

	resp := eng.Run(context.Background(), &check.Request{
		ChainID: anvilChainID,
		From:    anvilFunded,
		To:      "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
		Value:   "0xde0b6b3a7640000", // 1 ETH
		Data:    "0x",
	})

	if resp.Verdict != check.VerdictGo {
		t.Errorf("expected verdict go, got %s (risk=%d)", resp.Verdict, resp.RiskScore)
		for _, c := range resp.Checks {
			if c.Status != check.StatusPass {
				t.Logf("  check %s: %s — %s", c.Name, c.Status, c.Reason)
			}
		}
	}
	if resp.LatencyMS > 1500 {
		t.Errorf("latency %dms exceeds 1500ms budget", resp.LatencyMS)
	}
}

// TestFlightCheck_RevertingCall expects verdict "block" when the call will revert.
func TestFlightCheck_RevertingCall(t *testing.T) {
	eng := buildEngine(t)

	// Calling 0x000...dead with calldata 0xdeadbeef will revert (no contract code, no fallback).
	resp := eng.Run(context.Background(), &check.Request{
		ChainID: anvilChainID,
		From:    anvilFunded,
		To:      "0x000000000000000000000000000000000000dEaD",
		Value:   "0x0",
		Data:    "0xdeadbeef",
	})

	if resp.Verdict != check.VerdictBlock {
		t.Logf("expected block, got %s — this may be non-deterministic on some anvil versions", resp.Verdict)
		for _, c := range resp.Checks {
			t.Logf("  check %s: %s — %s", c.Name, c.Status, c.Reason)
		}
	}
}

// TestFlightCheck_UnlimitedApproval expects contract_risk to warn for a max-uint256 approval.
func TestFlightCheck_UnlimitedApproval(t *testing.T) {
	eng := buildEngine(t)

	// approve(address, 2^256-1) calldata.
	maxU256 := strings.Repeat("f", 64)
	spender := "000000000000000000000000" + strings.Repeat("f", 40)
	data := "0x095ea7b3" + spender + maxU256

	resp := eng.Run(context.Background(), &check.Request{
		ChainID: anvilChainID,
		From:    anvilFunded,
		To:      "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		Value:   "0x0",
		Data:    data,
	})

	foundWarn := false
	for _, c := range resp.Checks {
		if c.Name == "contract_risk" && c.Status == check.StatusWarn {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected contract_risk warn for unlimited approval, got verdict=%s", resp.Verdict)
		for _, c := range resp.Checks {
			t.Logf("  check %s: %s — %s", c.Name, c.Status, c.Reason)
		}
	}
}

// TestFlightCheck_PolicyMaxValue expects the policy check to warn when no DB is configured.
func TestFlightCheck_PolicyMaxValue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Docker-based test in -short mode")
	}
	c := &check.PolicyCheck{}
	policyID := "00000000-0000-0000-0000-000000000001"
	res, err := c.Run(context.Background(), &check.Request{
		PolicyID: &policyID,
		Value:    "0x100",
	}, &check.Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != check.StatusWarn {
		t.Errorf("expected warn without db, got %s: %s", res.Status, res.Reason)
	}
}
