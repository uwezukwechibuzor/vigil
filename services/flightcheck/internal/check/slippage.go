package check

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// SlippageCheck validates that the expected output is within the slippage tolerance.
type SlippageCheck struct{}

// Name returns the check name.
func (c *SlippageCheck) Name() string { return "slippage" }

// Run evaluates max_slippage_bps vs expected_out and also re-checks expired deadlines.
func (c *SlippageCheck) Run(_ context.Context, req *Request, _ *Deps) (*Result, error) {
	// If deadline already passed, escalate to fail here as well.
	if req.DeadlineS != nil {
		now := time.Now().Unix()
		if *req.DeadlineS < now {
			return &Result{
				Name:   c.Name(),
				Status: StatusFail,
				Reason: "slippage check skipped: transaction deadline already expired",
				Evidence: map[string]any{
					"deadline_s": *req.DeadlineS,
					"now_s":      now,
				},
			}, nil
		}
	}

	if req.MaxSlippageBPS <= 0 || req.ExpectedOut == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: "slippage check not applicable",
		}, nil
	}

	// Parse value (native change from transaction).
	var txValue *big.Int
	if req.Value != "" && req.Value != "0x0" && req.Value != "0x" {
		txValue = new(big.Int)
		valHex := strings.TrimPrefix(req.Value, "0x")
		if _, ok := txValue.SetString(valHex, 16); !ok {
			txValue = big.NewInt(0)
		}
	} else {
		txValue = big.NewInt(0)
	}

	// Parse expected_out.
	expectedOut := new(big.Int)
	expHex := strings.TrimPrefix(*req.ExpectedOut, "0x")
	if _, ok := expectedOut.SetString(expHex, 16); !ok {
		// Try decimal.
		if _, ok2 := expectedOut.SetString(*req.ExpectedOut, 10); !ok2 {
			return &Result{
				Name:     c.Name(),
				Status:   StatusWarn,
				Reason:   "could not parse expected_out value",
				Evidence: map[string]any{"expected_out": *req.ExpectedOut},
			}, nil
		}
	}

	if expectedOut.Sign() == 0 {
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: "expected_out is zero; slippage check skipped",
		}, nil
	}

	// slippage = (expectedOut - txValue) / expectedOut * 10000 bps
	// If txValue < expectedOut we have slippage > 0.
	diff := new(big.Int).Sub(expectedOut, txValue)
	if diff.Sign() < 0 {
		// actual > expected; that is favourable slippage — pass.
		return &Result{
			Name:   c.Name(),
			Status: StatusPass,
			Reason: "actual value exceeds expected; favourable",
		}, nil
	}

	// bps = diff * 10000 / expectedOut
	bps := new(big.Int).Mul(diff, big.NewInt(10000))
	bps.Div(bps, expectedOut)

	actualBPS := int(bps.Int64())
	if actualBPS > req.MaxSlippageBPS {
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("slippage %d bps exceeds max %d bps", actualBPS, req.MaxSlippageBPS),
			Evidence: map[string]any{
				"slippage_bps":     actualBPS,
				"max_slippage_bps": req.MaxSlippageBPS,
				"expected_out":     expectedOut.String(),
				"tx_value":         txValue.String(),
			},
		}, nil
	}

	return &Result{
		Name:   c.Name(),
		Status: StatusPass,
		Reason: fmt.Sprintf("slippage %d bps within limit %d bps", actualBPS, req.MaxSlippageBPS),
		Evidence: map[string]any{
			"slippage_bps":     actualBPS,
			"max_slippage_bps": req.MaxSlippageBPS,
		},
	}, nil
}
