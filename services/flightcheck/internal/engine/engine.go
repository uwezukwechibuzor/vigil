// Package engine runs all flight checks concurrently within a time budget.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// riskWeights maps (checkName, status) → score contribution.
var riskWeights = map[string]map[check.Status]int{
	"infra_health":     {check.StatusFail: 15, check.StatusWarn: 8},
	"simulation":       {check.StatusFail: 35, check.StatusWarn: 10},
	"oracle_freshness": {check.StatusFail: 5},
	"slippage":         {check.StatusFail: 10, check.StatusWarn: 5},
	"gas_sanity":       {check.StatusWarn: 5},
	"contract_risk":    {check.StatusFail: 15, check.StatusWarn: 8},
	"policy":           {check.StatusFail: 15},
	"address_safety":   {check.StatusFail: 10, check.StatusWarn: 5},
}

// ChainClient holds an ethclient for a specific chainID.
type ChainClient struct {
	ChainID uint64
	Client  *ethclient.Client
}

// Engine runs all registered checks concurrently with a time budget.
type Engine struct {
	registry *check.Registry
	clients  []ChainClient
	redis    goredis.UniversalClient
	pg       *dbpg.Pool
	logger   *slog.Logger
	budget   time.Duration
}

// New creates a new Engine.
func New(
	registry *check.Registry,
	clients []ChainClient,
	redis goredis.UniversalClient,
	pg *dbpg.Pool,
	logger *slog.Logger,
	budget time.Duration,
) *Engine {
	return &Engine{
		registry: registry,
		clients:  clients,
		redis:    redis,
		pg:       pg,
		logger:   logger,
		budget:   budget,
	}
}

// depsFor returns a Deps struct for the given chainID.
func (e *Engine) depsFor(chainID uint64) *check.Deps {
	var client *ethclient.Client
	for _, cc := range e.clients {
		if cc.ChainID == chainID {
			client = cc.Client
			break
		}
	}
	return &check.Deps{
		Client: client,
		Redis:  e.redis,
		PG:     e.pg,
		Logger: e.logger,
	}
}

// Run executes all registered checks concurrently and returns the aggregated response.
func (e *Engine) Run(ctx context.Context, req *check.Request) *check.Response {
	checkID := uuid.New().String()
	start := time.Now()

	// Apply budget deadline to context.
	budgetCtx, cancel := context.WithTimeout(ctx, e.budget)
	defer cancel()

	deps := e.depsFor(req.ChainID)
	checks := e.registry.All()
	results := make([]check.Result, len(checks))

	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, c := range checks {
		wg.Add(1)
		go func(i int, c check.Check) {
			defer wg.Done()

			r, err := c.Run(budgetCtx, req, deps)
			if err != nil || r == nil {
				reason := "check timed out or errored"
				if err != nil {
					reason = fmt.Sprintf("check error: %v", err)
				}
				r = &check.Result{
					Name:   c.Name(),
					Status: check.StatusWarn,
					Reason: reason,
				}
			}
			mu.Lock()
			results[i] = *r
			mu.Unlock()
		}(i, c)
	}

	wg.Wait()

	verdict, score := aggregateVerdict(results)
	balanceChanges := extractBalanceChanges(results)
	summary := buildSummary(verdict, results)

	return &check.Response{
		CheckID:        checkID,
		Verdict:        verdict,
		RiskScore:      score,
		Checks:         results,
		BalanceChanges: balanceChanges,
		Summary:        summary,
		LatencyMS:      time.Since(start).Milliseconds(),
	}
}

// aggregateVerdict computes the overall verdict and risk score.
func aggregateVerdict(results []check.Result) (check.Verdict, int) {
	verdict := check.VerdictGo
	score := 0

	for _, r := range results {
		switch r.Status {
		case check.StatusFail:
			verdict = check.VerdictBlock
		case check.StatusWarn:
			if verdict == check.VerdictGo {
				verdict = check.VerdictWarn
			}
		}

		if weights, ok := riskWeights[r.Name]; ok {
			score += weights[r.Status]
		}
	}

	if score > 100 {
		score = 100
	}
	return verdict, score
}

// extractBalanceChanges gathers balance changes from simulation evidence.
func extractBalanceChanges(results []check.Result) []check.BalanceChange {
	for _, r := range results {
		if r.Name != "simulation" {
			continue
		}
		raw, ok := r.Evidence["balance_changes"]
		if !ok {
			return nil
		}
		// Re-marshal to []BalanceChange.
		b, err := json.Marshal(raw)
		if err != nil {
			return nil
		}
		var changes []check.BalanceChange
		if err := json.Unmarshal(b, &changes); err != nil {
			return nil
		}
		return changes
	}
	return nil
}

// buildSummary writes a human-readable verdict paragraph.
func buildSummary(verdict check.Verdict, results []check.Result) string {
	var failNames, warnNames []string
	for _, r := range results {
		switch r.Status {
		case check.StatusFail:
			failNames = append(failNames, r.Name)
		case check.StatusWarn:
			warnNames = append(warnNames, r.Name)
		}
	}

	switch verdict {
	case check.VerdictBlock:
		return fmt.Sprintf(
			"Transaction is blocked. Failed checks: %s. Vigil gives risk signals, never guarantees.",
			strings.Join(failNames, ", "),
		)
	case check.VerdictWarn:
		return fmt.Sprintf(
			"Transaction may proceed with caution. Warnings from: %s. Vigil gives risk signals, never guarantees.",
			strings.Join(warnNames, ", "),
		)
	default:
		return "All checks passed. Vigil gives risk signals, never guarantees."
	}
}
