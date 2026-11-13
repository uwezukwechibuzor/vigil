// Package adapter defines the chain-agnostic interface Vigil uses to query any blockchain.
// EVM is the first implementation; Solana can be added by implementing this interface.
package adapter

import (
	"context"
	"math/big"
	"time"
)

// Block is a minimal block representation.
type Block struct {
	Number  uint64
	Hash    string
	Time    time.Time
	BaseFee *big.Int // nil on pre-EIP-1559 chains
}

// FeeData holds current gas pricing info.
type FeeData struct {
	BaseFee        *big.Int         // EIP-1559 base fee; nil for legacy
	MaxFee         *big.Int         // suggested maxFeePerGas
	PriorityFee    *big.Int         // suggested maxPriorityFeePerGas
	GasPrice       *big.Int         // legacy gasPrice fallback
	PriorityFeePct map[int]*big.Int // percentiles (10,50,90)
}

// Log is an EVM event log.
type Log struct {
	Address     string
	Topics      []string
	Data        []byte
	BlockNumber uint64
	TxHash      string
	Index       uint
}

// CallMsg is a minimal eth_call / EstimateGas payload.
type CallMsg struct {
	From  string
	To    string
	Data  []byte
	Value *big.Int
}

// FilterQuery selects logs.
type FilterQuery struct {
	FromBlock *big.Int
	ToBlock   *big.Int
	Addresses []string
	Topics    [][]string
}

// ChainAdapter is the interface every chain implementation must satisfy.
// Implementations must be safe for concurrent use.
type ChainAdapter interface {
	// HeadBlock returns the current canonical head.
	HeadBlock(ctx context.Context) (*Block, error)

	// BlockByNumber returns a specific block; nil fetches the latest.
	BlockByNumber(ctx context.Context, number *big.Int) (*Block, error)

	// Call performs a read-only contract call.
	Call(ctx context.Context, msg CallMsg) ([]byte, error)

	// EstimateGas estimates the gas cost of a transaction.
	EstimateGas(ctx context.Context, msg CallMsg) (uint64, error)

	// FeeData returns current gas fee market data.
	FeeData(ctx context.Context) (*FeeData, error)

	// Simulate executes a call and returns the result without committing state.
	Simulate(ctx context.Context, msg CallMsg) ([]byte, error)

	// GetLogs returns logs matching the filter.
	GetLogs(ctx context.Context, q FilterQuery) ([]Log, error)

	// ChainID returns the numeric chain ID.
	ChainID(ctx context.Context) (uint64, error)

	// Close releases any persistent connections.
	Close()
}
