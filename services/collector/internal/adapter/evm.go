package adapter

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EVMAdapter implements ChainAdapter for EVM-compatible chains via go-ethereum.
// It supports both HTTP and WebSocket RPC URLs.
type EVMAdapter struct {
	client *ethclient.Client
	url    string
}

// NewEVM dials the given RPC URL (http://, https://, ws://, wss://) and returns an EVMAdapter.
func NewEVM(ctx context.Context, rpcURL string) (*EVMAdapter, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("evm dial %s: %w", rpcURL, err)
	}
	return &EVMAdapter{client: client, url: rpcURL}, nil
}

// Close closes the underlying RPC connection.
func (e *EVMAdapter) Close() {
	e.client.Close()
}

// ChainID returns the network chain ID.
func (e *EVMAdapter) ChainID(ctx context.Context) (uint64, error) {
	id, err := e.client.ChainID(ctx)
	if err != nil {
		return 0, fmt.Errorf("chainid: %w", err)
	}
	return id.Uint64(), nil
}

// HeadBlock returns the latest canonical head block.
func (e *EVMAdapter) HeadBlock(ctx context.Context) (*Block, error) {
	return e.BlockByNumber(ctx, nil)
}

// BlockByNumber fetches a block by number; nil fetches the latest.
func (e *EVMAdapter) BlockByNumber(ctx context.Context, number *big.Int) (*Block, error) {
	b, err := e.client.BlockByNumber(ctx, number)
	if err != nil {
		return nil, fmt.Errorf("block by number: %w", err)
	}
	return evmBlockToBlock(b), nil
}

// Call performs a read-only eth_call.
func (e *EVMAdapter) Call(ctx context.Context, msg CallMsg) ([]byte, error) {
	result, err := e.client.CallContract(ctx, toEthMsg(msg), nil)
	if err != nil {
		return nil, fmt.Errorf("eth_call: %w", err)
	}
	return result, nil
}

// Simulate is equivalent to Call for EVM — no pending-state distinction.
func (e *EVMAdapter) Simulate(ctx context.Context, msg CallMsg) ([]byte, error) {
	return e.Call(ctx, msg)
}

// EstimateGas estimates the gas required for a transaction.
func (e *EVMAdapter) EstimateGas(ctx context.Context, msg CallMsg) (uint64, error) {
	gas, err := e.client.EstimateGas(ctx, toEthMsg(msg))
	if err != nil {
		return 0, fmt.Errorf("estimate_gas: %w", err)
	}
	return gas, nil
}

// FeeData returns EIP-1559 fee market data plus priority fee percentiles.
func (e *EVMAdapter) FeeData(ctx context.Context) (*FeeData, error) {
	head, err := e.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}

	fd := &FeeData{
		BaseFee:        head.BaseFee,
		PriorityFeePct: make(map[int]*big.Int),
	}

	// Suggest tip (priority fee) for the next block
	tip, err := e.client.SuggestGasTipCap(ctx)
	if err == nil {
		fd.PriorityFee = tip
		fd.PriorityFeePct[50] = new(big.Int).Set(tip)
	}

	// Suggest full price (legacy / EIP-1559 combined)
	gp, err := e.client.SuggestGasPrice(ctx)
	if err == nil {
		fd.GasPrice = gp
	}

	if fd.BaseFee != nil && fd.PriorityFee != nil {
		fd.MaxFee = new(big.Int).Add(
			new(big.Int).Mul(fd.BaseFee, big.NewInt(2)),
			fd.PriorityFee,
		)
	}

	// Pull reward percentiles from eth_feeHistory (10th, 50th, 90th)
	history, err := e.client.FeeHistory(ctx, 5, nil, []float64{10, 50, 90})
	if err == nil && len(history.Reward) > 0 {
		last := history.Reward[len(history.Reward)-1]
		pcts := []int{10, 50, 90}
		for i, pct := range pcts {
			if i < len(last) && last[i] != nil {
				fd.PriorityFeePct[pct] = new(big.Int).Set(last[i])
			}
		}
	}

	return fd, nil
}

// GetLogs returns event logs matching the filter query.
func (e *EVMAdapter) GetLogs(ctx context.Context, q FilterQuery) ([]Log, error) {
	ethQ := ethereum.FilterQuery{
		FromBlock: q.FromBlock,
		ToBlock:   q.ToBlock,
	}
	for _, a := range q.Addresses {
		ethQ.Addresses = append(ethQ.Addresses, common.HexToAddress(a))
	}
	for _, row := range q.Topics {
		var t []common.Hash
		for _, h := range row {
			t = append(t, common.HexToHash(h))
		}
		ethQ.Topics = append(ethQ.Topics, t)
	}

	raw, err := e.client.FilterLogs(ctx, ethQ)
	if err != nil {
		return nil, fmt.Errorf("filter_logs: %w", err)
	}

	out := make([]Log, len(raw))
	for i, l := range raw {
		out[i] = evmLogToLog(l)
	}
	return out, nil
}

// URL returns the RPC endpoint URL this adapter is connected to.
func (e *EVMAdapter) URL() string { return e.url }

// IsWebSocket reports whether this adapter uses a WebSocket connection.
func (e *EVMAdapter) IsWebSocket() bool {
	return strings.HasPrefix(e.url, "ws://") || strings.HasPrefix(e.url, "wss://")
}

// --- helpers ----------------------------------------------------------------

func evmBlockToBlock(b *ethtypes.Block) *Block {
	return &Block{
		Number:  b.NumberU64(),
		Hash:    b.Hash().Hex(),
		Time:    time.Unix(int64(b.Time()), 0).UTC(), // #nosec G115 -- block timestamp (seconds since epoch) fits int64 for all historical/near-future blocks
		BaseFee: b.BaseFee(),
	}
}

func evmLogToLog(l ethtypes.Log) Log {
	topics := make([]string, len(l.Topics))
	for i, t := range l.Topics {
		topics[i] = t.Hex()
	}
	return Log{
		Address:     l.Address.Hex(),
		Topics:      topics,
		Data:        l.Data,
		BlockNumber: l.BlockNumber,
		TxHash:      l.TxHash.Hex(),
		Index:       l.Index,
	}
}

func toEthMsg(msg CallMsg) ethereum.CallMsg {
	m := ethereum.CallMsg{Data: msg.Data, Value: msg.Value}
	if msg.From != "" {
		m.From = common.HexToAddress(msg.From)
	}
	if msg.To != "" {
		to := common.HexToAddress(msg.To)
		m.To = &to
	}
	return m
}
