package check

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// selectors for well-known ERC-20 functions.
const (
	selectorTransfer     = "a9059cbb" // transfer(address,uint256)
	selectorTransferFrom = "23b872dd" // transferFrom(address,address,uint256)
	selectorError        = "08c379a0" // Error(string)
)

// SimulationCheck simulates the transaction via eth_call and detects reverts.
type SimulationCheck struct{}

// Name returns the check name.
func (c *SimulationCheck) Name() string { return "simulation" }

// Run performs eth_call and analyses the result.
func (c *SimulationCheck) Run(ctx context.Context, req *Request, deps *Deps) (*Result, error) {
	if deps.Client == nil {
		return &Result{
			Name:   c.Name(),
			Status: StatusWarn,
			Reason: "no RPC client; simulation skipped",
		}, nil
	}

	from := common.HexToAddress(req.From)
	to := common.HexToAddress(req.To)

	var value *big.Int
	if req.Value != "" && req.Value != "0x0" && req.Value != "0x" {
		value = new(big.Int)
		valHex := strings.TrimPrefix(req.Value, "0x")
		if _, ok := value.SetString(valHex, 16); !ok {
			value = big.NewInt(0)
		}
	} else {
		value = big.NewInt(0)
	}

	var data []byte
	if req.Data != "" && req.Data != "0x" {
		decoded, err := hex.DecodeString(strings.TrimPrefix(req.Data, "0x"))
		if err == nil {
			data = decoded
		}
	}

	msg := ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: value,
		Data:  data,
	}

	result, err := deps.Client.CallContract(ctx, msg, nil)
	if err != nil {
		reason := decodeRevertReason(err.Error())
		return &Result{
			Name:   c.Name(),
			Status: StatusFail,
			Reason: fmt.Sprintf("transaction would revert: %s", reason),
			Evidence: map[string]any{
				"revert_reason": reason,
				"raw_error":     err.Error(),
			},
		}, nil
	}

	// Check if return data itself is a revert reason (some nodes return 0x08c379a0 on success path).
	if revertMsg := decodeRevertFromBytes(result); revertMsg != "" {
		return &Result{
			Name:     c.Name(),
			Status:   StatusFail,
			Reason:   fmt.Sprintf("simulation returned revert: %s", revertMsg),
			Evidence: map[string]any{"revert_reason": revertMsg},
		}, nil
	}

	evidence := map[string]any{
		"return_data_hex": hex.EncodeToString(result),
	}

	balanceChanges := extractBalanceChanges(req, data)
	if len(balanceChanges) > 0 {
		changes := make([]map[string]string, 0, len(balanceChanges))
		for _, bc := range balanceChanges {
			changes = append(changes, map[string]string{
				"token":   bc.Token,
				"account": bc.Account,
				"delta":   bc.Delta,
			})
		}
		evidence["balance_changes"] = changes
	}

	return &Result{
		Name:     c.Name(),
		Status:   StatusPass,
		Reason:   "simulation succeeded",
		Evidence: evidence,
	}, nil
}

// decodeRevertReason extracts a human-readable reason from an RPC error string.
func decodeRevertReason(errStr string) string {
	// Look for hex-encoded revert data embedded in RPC error.
	idx := strings.Index(errStr, "0x")
	if idx == -1 {
		return errStr
	}
	hexPart := strings.Fields(errStr[idx:])[0]
	hexPart = strings.TrimPrefix(hexPart, "0x")
	b, err := hex.DecodeString(hexPart)
	if err != nil || len(b) < 4 {
		return errStr
	}
	if msg := decodeRevertFromBytes(b); msg != "" {
		return msg
	}
	return errStr
}

// decodeRevertFromBytes attempts to ABI-decode an Error(string) revert payload.
func decodeRevertFromBytes(b []byte) string {
	if len(b) < 4 {
		return ""
	}
	selector := hex.EncodeToString(b[:4])
	if selector != selectorError {
		return ""
	}
	// ABI: bytes 4..36 = offset, bytes 36..68 = length, bytes 68.. = string data
	if len(b) < 68 {
		return ""
	}
	length := binary.BigEndian.Uint32(b[64:68])
	end := 68 + int(length)
	if end > len(b) {
		end = len(b)
	}
	return string(b[68:end])
}

// extractBalanceChanges parses ERC-20 transfer selectors from calldata.
func extractBalanceChanges(req *Request, data []byte) []BalanceChange {
	if len(data) < 4 {
		return nil
	}
	selector := hex.EncodeToString(data[:4])

	switch selector {
	case selectorTransfer:
		// transfer(address to, uint256 amount) → 4 + 32 + 32 = 68 bytes
		if len(data) < 68 {
			return nil
		}
		toAddr := common.BytesToAddress(data[4:36])
		amount := new(big.Int).SetBytes(data[36:68])
		return []BalanceChange{
			{Token: req.To, Account: req.From, Delta: "-" + amount.String()},
			{Token: req.To, Account: toAddr.Hex(), Delta: amount.String()},
		}
	case selectorTransferFrom:
		// transferFrom(address from, address to, uint256 amount) → 4 + 32 + 32 + 32 = 100 bytes
		if len(data) < 100 {
			return nil
		}
		fromAddr := common.BytesToAddress(data[4:36])
		toAddr := common.BytesToAddress(data[36:68])
		amount := new(big.Int).SetBytes(data[68:100])
		return []BalanceChange{
			{Token: req.To, Account: fromAddr.Hex(), Delta: "-" + amount.String()},
			{Token: req.To, Account: toAddr.Hex(), Delta: amount.String()},
		}
	}
	return nil
}
