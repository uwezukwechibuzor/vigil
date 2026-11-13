package check

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// PolicyCheck validates the transaction against a named policy stored in Postgres.
type PolicyCheck struct{}

// Name returns the check name.
func (c *PolicyCheck) Name() string { return "policy" }

// Run loads the policy (if any) and validates chain, denied contracts, and max value.
func (c *PolicyCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	if req.PolicyID == nil || *req.PolicyID == "" {
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: "no policy specified",
		}, nil
	}

	// Validate UUID format first (before any DB call).
	policyID, err := uuid.Parse(*req.PolicyID)
	if err != nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("invalid policy_id format: %v", err),
		}, nil
	}

	if deps.PG == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusWarn,
			Reason: "policy check skipped: no database connection",
		}, nil
	}

	policy, err := deps.PG.GetPolicy(ctx, policyID)
	if err != nil {
		if errors.Is(err, dbpg.ErrPolicyNotFound) {
			return &Result{
				Name:   c.Name(),
				Status: StatusFail,
				Reason: fmt.Sprintf("policy %s not found", *req.PolicyID),
			}, nil
		}
		return &Result{
			Name:   c.Name(),
			Status: StatusWarn,
			Reason: fmt.Sprintf("policy lookup error: %v", err),
		}, nil
	}

	if !policy.Enabled {
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: fmt.Sprintf("policy %q is disabled; check skipped", policy.Name),
		}, nil
	}

	rules := policy.Rules
	var violations []string

	// 1. Check allowed_chains.
	if len(rules.AllowedChains) > 0 {
		allowed := false
		for _, id := range rules.AllowedChains {
			if id == req.ChainID {
				allowed = true
				break
			}
		}
		if !allowed {
			violations = append(violations, fmt.Sprintf("chain %d not in policy allowed_chains", req.ChainID))
		}
	}

	// 2. Check denied_contracts.
	for _, denied := range rules.DeniedContracts {
		if strings.EqualFold(denied, req.To) {
			violations = append(violations, fmt.Sprintf("contract %s is denied by policy", req.To))
		}
	}

	// 3. Check max_value_wei.
	if rules.MaxValueWei != "" {
		maxVal := new(big.Int)
		if _, ok := maxVal.SetString(rules.MaxValueWei, 10); ok && maxVal.Sign() > 0 {
			var txVal *big.Int
			if req.Value != "" && req.Value != "0x0" && req.Value != "0x" {
				txVal = new(big.Int)
				valHex := strings.TrimPrefix(req.Value, "0x")
				if _, ok2 := txVal.SetString(valHex, 16); !ok2 {
					txVal = big.NewInt(0)
				}
			} else {
				txVal = big.NewInt(0)
			}
			if txVal.Cmp(maxVal) > 0 {
				violations = append(violations, fmt.Sprintf(
					"value %s wei exceeds policy max %s wei", txVal.String(), maxVal.String()))
			}
		}
	}

	if len(violations) > 0 {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: strings.Join(violations, "; "),
			Evidence: map[string]any{
				"policy_id":   policy.ID.String(),
				"policy_name": policy.Name,
				"violations":  violations,
			},
		}, nil
	}

	return &Result{
		Name:   c.Name(),
		Status: StatusPass,
		Reason: fmt.Sprintf("transaction passes policy %q", policy.Name),
		Evidence: map[string]any{
			"policy_id":   policy.ID.String(),
			"policy_name": policy.Name,
		},
	}, nil
}
