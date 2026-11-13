// Package linker fetches onchain receipts and links spans to confirmed transactions.
package linker

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/uwezukwechibuzor/vigil/services/ingest/internal/cost"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
)

const (
	queueCap   = 10_000
	maxRetries = 10
	retryDelay = 2 * time.Second
)

// Linker is a worker that fetches onchain receipts and links spans.
type Linker struct {
	ch      *dbch.Client
	clients map[uint64]*ethclient.Client // chain_id → RPC client
	logger  *slog.Logger
	queue   chan dbch.Span
	pricer  cost.GasPricer // may be nil
}

// New creates a new Linker.
func New(ch *dbch.Client, clients map[uint64]*ethclient.Client, pricer cost.GasPricer, logger *slog.Logger) *Linker {
	return &Linker{
		ch:      ch,
		clients: clients,
		pricer:  pricer,
		logger:  logger,
		queue:   make(chan dbch.Span, queueCap),
	}
}

// Enqueue adds a span to the linking queue (non-blocking, drops if full).
func (l *Linker) Enqueue(span dbch.Span) {
	select {
	case l.queue <- span:
	default:
		l.logger.Warn("linker queue full, dropping span", "trace_id", span.TraceID)
	}
}

// Run processes the linking queue until ctx is cancelled.
func (l *Linker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case span := <-l.queue:
			l.link(ctx, span)
		}
	}
}

// link fetches receipt and marks the span linked, retrying up to maxRetries times.
func (l *Linker) link(ctx context.Context, span dbch.Span) {
	client, ok := l.clients[span.VigilChainID]
	if !ok {
		l.logger.Warn("no rpc client for chain", "chain_id", span.VigilChainID)
		return
	}

	txHash := common.HexToHash(span.VigilTxHash)
	var receipt *types.Receipt

	for attempt := 0; attempt < maxRetries; attempt++ {
		var err error
		receipt, err = client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(retryDelay):
		}
	}

	if receipt == nil {
		l.logger.Warn("could not fetch receipt after retries", "tx_hash", span.VigilTxHash)
		return
	}

	gasEstimate := uint64(0)
	if est, ok := span.Attrs["vigil.gas_estimate"]; ok {
		if n, err := strconv.ParseUint(est, 10, 64); err == nil {
			gasEstimate = n
		}
	}

	var sf uint8
	if IsSilentFailure(receipt, gasEstimate) {
		sf = 1
	}

	var gasCostUSD float64
	if l.pricer != nil {
		if ethPrice, err := l.pricer.ETHPriceUSD(ctx); err == nil {
			gasCostUSD = cost.GasCostUSD(receipt.GasUsed, receipt.EffectiveGasPrice, ethPrice)
		}
	}

	if err := l.ch.MarkSpanLinked(ctx, span.TraceID, span.SpanID, sf, gasCostUSD); err != nil {
		l.logger.Error("mark span linked failed", "trace_id", span.TraceID, "error", err)
	}
}

// IsSilentFailure checks if a confirmed tx result differs from intent/simulation.
// A silent failure is: receipt.Status==1 AND (gas_used > 2× estimate OR no events when expected).
// Exported for unit testing.
func IsSilentFailure(receipt *types.Receipt, gasEstimate uint64) bool {
	if receipt == nil || receipt.Status != 1 {
		return false
	}
	if gasEstimate > 0 && receipt.GasUsed > 2*gasEstimate {
		return true
	}
	return false
}
