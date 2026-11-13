//go:build ignore

// Example: guardedSend with a custom signer on Base Sepolia.
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"

	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

// stubSigner is a placeholder — replace with go-ethereum TransactOpts.
type stubSigner struct{}

func (s *stubSigner) SendTransaction(_ context.Context, tx vigil.TransactionRequest) (string, error) {
	fmt.Printf("Would send tx to %s value=%v\n", tx.To, tx.Value)
	return "0xtxhash", nil
}

func main() {
	client := vigil.New()
	signer := &stubSigner{}

	txHash, fcID, err := vigil.GuardedSend(
		context.Background(),
		client,
		signer,
		vigil.FlightCheckRequest{
			ChainID: 84532, // Base Sepolia
			From:    "0xYourAddress",
			To:      "0xContractAddress",
			Data:    "0x",
			Value:   "0x0",
		},
		vigil.GuardedSendOpts{
			OnWarn: func(r *vigil.FlightCheckResult) error {
				fmt.Printf("warn: score=%.2f\n", r.Score)
				return nil
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("tx: %s fc: %s\n", txHash, fcID)

	_ = big.NewInt(0) // ensure math/big is used
}
