// Package check defines the types, interfaces and check implementations for the flight check engine.
package check

import (
	"log/slog"

	"github.com/ethereum/go-ethereum/ethclient"
	goredis "github.com/redis/go-redis/v9"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// Verdict is the overall flight-check decision.
type Verdict string

// Verdict values.
const (
	VerdictGo    Verdict = "go"
	VerdictWarn  Verdict = "warn"
	VerdictBlock Verdict = "block"
)

// Status is the per-check outcome.
type Status string

// Status values.
const (
	StatusPass Status = "pass"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Request is the inbound flight-check payload.
type Request struct {
	ChainID        uint64  `json:"chain_id"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Data           string  `json:"data"`
	Value          string  `json:"value"`
	Gas            *uint64 `json:"gas"`
	Intent         string  `json:"intent"`
	PolicyID       *string `json:"policy_id"`
	MaxSlippageBPS int     `json:"max_slippage_bps"`
	ExpectedOut    *string `json:"expected_out"`
	DeadlineS      *int64  `json:"deadline_s"`
	// ProjectID is optional; enables policy loading and usage tracking.
	ProjectID string `json:"project_id"`
}

// Result is the outcome of a single check.
type Result struct {
	Name     string         `json:"name"`
	Status   Status         `json:"status"`
	Reason   string         `json:"reason"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// BalanceChange describes a token balance delta inferred from simulation.
type BalanceChange struct {
	Token   string `json:"token"`
	Account string `json:"account"`
	Delta   string `json:"delta"`
}

// Response is the full flight-check response returned to callers.
type Response struct {
	CheckID        string          `json:"check_id"`
	Verdict        Verdict         `json:"verdict"`
	RiskScore      int             `json:"risk_score"`
	Checks         []Result        `json:"checks"`
	BalanceChanges []BalanceChange `json:"balance_changes,omitempty"`
	Summary        string          `json:"summary"`
	LatencyMS      int64           `json:"latency_ms"`
}

// Deps holds external dependencies passed to every check.
type Deps struct {
	Client *ethclient.Client
	Redis  goredis.UniversalClient
	PG     *dbpg.Pool
	Logger *slog.Logger
}
