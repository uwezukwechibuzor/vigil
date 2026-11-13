package vigil

import (
	"context"
	"fmt"
	"math/big"
)

// TransactionRequest is a raw EVM transaction to send.
type TransactionRequest struct {
	To       string
	Data     []byte
	Value    *big.Int
	GasLimit uint64
}

// Signer abstracts sending a signed transaction on-chain.
type Signer interface {
	// SendTransaction signs and broadcasts the transaction, returning the tx hash.
	SendTransaction(ctx context.Context, tx TransactionRequest) (txHash string, err error)
}

// GuardedSendOpts are options for GuardedSend.
type GuardedSendOpts struct {
	// OnWarn is called when the flight check verdict is "warn". Returning an error aborts the send.
	OnWarn func(result *FlightCheckResult) error
	// Run attaches the tx hash to this run span after sending.
	Run *Run
}

// GuardedSend runs a flight check and sends the transaction only if the verdict is not "fail".
// Returns (txHash, flightCheckID, error).
// Never returns an error for "warn" verdict — call opts.OnWarn to handle warnings.
func GuardedSend(
	ctx context.Context,
	client *Client,
	signer Signer,
	req FlightCheckRequest,
	opts GuardedSendOpts,
) (string, string, error) {
	result, err := client.FlightCheck(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("vigil: flight check: %w", err)
	}

	if result.Verdict == VerdictFail {
		return "", result.ID, fmt.Errorf("vigil: flight check failed (score=%.2f)", result.Score)
	}

	if result.Verdict == VerdictWarn && opts.OnWarn != nil {
		if warnErr := opts.OnWarn(result); warnErr != nil {
			return "", result.ID, fmt.Errorf("vigil: warn handler aborted send: %w", warnErr)
		}
	}

	var value *big.Int
	if req.Value != "" && req.Value != "0x0" {
		v, ok := new(big.Int).SetString(req.Value, 0)
		if !ok {
			return "", result.ID, fmt.Errorf("vigil: invalid tx value: %s", req.Value)
		}
		value = v
	}

	txReq := TransactionRequest{
		To:    req.To,
		Value: value,
	}

	txHash, err := signer.SendTransaction(ctx, txReq)
	if err != nil {
		return "", result.ID, fmt.Errorf("vigil: send transaction: %w", err)
	}

	if opts.Run != nil {
		opts.Run.AttachTx(txHash, result.ID)
	}

	return txHash, result.ID, nil
}
