package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IndexerProbe measures lag for a generic subgraph / HTTP indexer by querying
// the latest indexed block number and comparing it to the chain head.
type IndexerProbe struct {
	endpointID string
	indexerURL string
	chainHead  func(ctx context.Context) (uint64, error)
	httpClient *http.Client
}

// NewIndexer creates an indexer lag probe.
// chainHead is a function that returns the current chain head block number.
func NewIndexer(endpointID, indexerURL string, chainHead func(ctx context.Context) (uint64, error)) *IndexerProbe {
	return &IndexerProbe{
		endpointID: endpointID,
		indexerURL: indexerURL,
		chainHead:  chainHead,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Kind returns KindIndexer.
func (p *IndexerProbe) Kind() Kind { return KindIndexer }

// Run queries the indexer's latest block and computes block lag.
func (p *IndexerProbe) Run(ctx context.Context) (*Result, error) {
	r := &Result{
		EndpointID: p.endpointID,
		Kind:       KindIndexer,
		Ts:         time.Now().UTC(),
		Labels:     map[string]string{"url": p.indexerURL},
	}

	indexedBlock, err := p.queryIndexedBlock(ctx)
	if err != nil {
		r.OK = false
		r.Err = fmt.Errorf("indexer query: %w", err)
		return r, r.Err
	}

	chainBlock, err := p.chainHead(ctx)
	if err != nil {
		// Can't determine lag without chain head, but indexer is up.
		r.OK = true
		r.Value = 0
		r.Labels["indexed_block"] = fmt.Sprintf("%d", indexedBlock)
		return r, nil
	}

	lag := IndexerLag(chainBlock, indexedBlock)
	r.OK = true
	r.Value = float64(lag)
	r.Labels["indexed_block"] = fmt.Sprintf("%d", indexedBlock)
	r.Labels["chain_block"] = fmt.Sprintf("%d", chainBlock)
	r.Labels["lag_blocks"] = fmt.Sprintf("%d", lag)
	return r, nil
}

// IndexerLag computes the block lag between chain head and the last indexed block.
// Returns 0 if the indexer is ahead (shouldn't happen but is not an error).
func IndexerLag(chainHead, indexedBlock uint64) uint64 {
	if chainHead <= indexedBlock {
		return 0
	}
	return chainHead - indexedBlock
}

// subgraphQuery is the GraphQL query to fetch the latest indexed block.
const subgraphQuery = `{"query":"{_meta{block{number}}}"}`

func (p *IndexerProbe) queryIndexedBlock(ctx context.Context) (uint64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.indexerURL,
		strings.NewReader(subgraphQuery))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("indexer returned %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Meta struct {
				Block struct {
					Number uint64 `json:"number"`
				} `json:"block"`
			} `json:"_meta"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse indexer response: %w", err)
	}
	return result.Data.Meta.Block.Number, nil
}
