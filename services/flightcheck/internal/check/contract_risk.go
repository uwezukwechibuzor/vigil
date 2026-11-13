package check

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
)

const (
	selectorApprove = "095ea7b3" // approve(address,uint256)
)

// maxUint256 is 2^256 - 1, the canonical unlimited approval amount.
var maxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// ContractRiskCheck verifies contract existence and flags risky calldata patterns.
type ContractRiskCheck struct{}

// Name returns the check name.
func (c *ContractRiskCheck) Name() string { return "contract_risk" }

// Run checks contract code existence, approve patterns, and project denylist.
func (c *ContractRiskCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	toAddr := common.HexToAddress(req.To)
	evidence := map[string]any{}
	var warnings []string
	var fails []string

	// 1. eth_getCode — check the contract exists.
	if deps.Client != nil {
		code, err := deps.Client.CodeAt(ctx, toAddr, nil)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("could not fetch contract code: %v", err))
		} else {
			evidence["code_size"] = len(code)
			if len(code) == 0 {
				evidence["is_eoa"] = true
			}
		}
	}

	// 2. Parse calldata selector.
	var data []byte
	if req.Data != "" && req.Data != "0x" {
		decoded, err := hex.DecodeString(strings.TrimPrefix(req.Data, "0x"))
		if err == nil {
			data = decoded
		}
	}

	if len(data) >= 4 {
		selector := hex.EncodeToString(data[:4])
		evidence["selector"] = "0x" + selector

		if selector == selectorApprove && len(data) >= 68 {
			// approve(address spender, uint256 amount) → 4 + 32 + 32 = 68
			amount := new(big.Int).SetBytes(data[36:68])
			evidence["approve_amount"] = amount.String()
			if amount.Cmp(maxUint256) == 0 {
				warnings = append(warnings, "unlimited ERC-20 approval (max uint256)")
			}
		}
	}

	// 3. Check denylist (requires project context).
	if deps.PG != nil && req.ProjectID != "" {
		projectID, err := uuid.Parse(req.ProjectID)
		if err == nil {
			denied, err := deps.PG.IsAddressDenied(ctx, projectID, req.To)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("denylist check failed: %v", err))
			} else if denied {
				fails = append(fails, fmt.Sprintf("address %s is on project denylist", req.To))
				evidence["denied"] = true
			}
		}
	}

	if len(fails) > 0 {
		return &Result{
			Name:     c.Name(),
			Status:   StatusFail,
			Reason:   fails[0],
			Evidence: evidence,
		}, nil
	}
	if len(warnings) > 0 {
		return &Result{
			Name:     c.Name(),
			Status:   StatusWarn,
			Reason:   strings.Join(warnings, "; "),
			Evidence: evidence,
		}, nil
	}

	return &Result{
		Name:     c.Name(),
		Status:   StatusPass,
		Reason:   "no contract risks detected",
		Evidence: evidence,
	}, nil
}
