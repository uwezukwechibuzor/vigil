package vigil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.vigil.dev"
	maxRetries     = 3
	userAgent      = "vigil-go/0.1.0"
)

// Client is the Vigil API client.
type Client struct {
	apiKey  string
	baseURL string
	hc      *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithAPIKey sets the API key.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithBaseURL overrides the default API base URL.
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient provides a custom *http.Client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.hc = hc }
}

// New creates a Client. If no API key is provided, reads VIGIL_API_KEY env var.
// Base URL defaults to https://api.vigil.dev.
func New(opts ...Option) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	if c.apiKey == "" {
		c.apiKey = os.Getenv("VIGIL_API_KEY")
	}
	return c
}

// APIError is returned when the Vigil API returns an error response.
type APIError struct {
	Code       string
	Message    string
	Details    map[string]interface{}
	StatusCode int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("vigil: %s (%s, HTTP %d)", e.Message, e.Code, e.StatusCode)
}

// ---- Core HTTP ----

func newUUID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(rand.IntN(256)) //nolint:gosec
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (c *Client) buildURL(path string, query url.Values) string {
	u := c.baseURL + "/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("vigil: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	isPost := method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch
	idempotencyKey := ""
	if isPost {
		idempotencyKey = newUUID()
	}

	targetURL := c.buildURL(path, query)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		}

		req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
		if err != nil {
			return fmt.Errorf("vigil: new request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", userAgent)
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		if idempotencyKey != "" {
			req.Header.Set("X-Idempotency-Key", idempotencyKey)
		}

		resp, err := c.hc.Do(req)
		if err != nil {
			return fmt.Errorf("vigil: http: %w", err)
		}

		shouldRetry := (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < maxRetries
		if shouldRetry {
			_ = resp.Body.Close()
			wait := retryWait(attempt, resp.Header.Get("Retry-After"))
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusNoContent {
			return nil
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("vigil: read body: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var errResp struct {
				Error struct {
					Code    string                 `json:"code"`
					Message string                 `json:"message"`
					Details map[string]interface{} `json:"details"`
				} `json:"error"`
			}
			if jsonErr := json.Unmarshal(respBody, &errResp); jsonErr == nil && errResp.Error.Code != "" {
				return &APIError{
					Code:       errResp.Error.Code,
					Message:    errResp.Error.Message,
					Details:    errResp.Error.Details,
					StatusCode: resp.StatusCode,
				}
			}
			return &APIError{
				Code:       "api_error",
				Message:    fmt.Sprintf("HTTP %d", resp.StatusCode),
				StatusCode: resp.StatusCode,
			}
		}

		if out != nil {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("vigil: decode response: %w", err)
			}
		}
		return nil
	}

	return &APIError{Code: "max_retries", Message: "max retries exceeded", StatusCode: 0}
}

func retryWait(attempt int, retryAfterHeader string) time.Duration {
	if retryAfterHeader != "" {
		if s, err := strconv.ParseFloat(retryAfterHeader, 64); err == nil {
			return time.Duration(s*1000) * time.Millisecond
		}
	}
	base := math.Pow(2, float64(attempt)) * 1000
	jitter := float64(rand.IntN(100)) //nolint:gosec
	ms := base + jitter
	if ms > 30000 {
		ms = 30000
	}
	return time.Duration(ms) * time.Millisecond
}

// ---- Flight Checks ----

// FlightCheck runs a pre-transaction flight check.
// Never returns an error on "warn" verdict — check result.Verdict.
func (c *Client) FlightCheck(ctx context.Context, req FlightCheckRequest) (*FlightCheckResult, error) {
	var result FlightCheckResult
	if err := c.do(ctx, http.MethodPost, "/flight-checks", nil, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetFlightCheck retrieves a flight check result by ID.
func (c *Client) GetFlightCheck(ctx context.Context, id string) (*FlightCheckResult, error) {
	var result FlightCheckResult
	if err := c.do(ctx, http.MethodGet, "/flight-checks/"+id, nil, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---- Chains ----

// ListChains returns all supported chains.
func (c *Client) ListChains(ctx context.Context) ([]Chain, error) {
	var list ChainList
	if err := c.do(ctx, http.MethodGet, "/chains", nil, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// ChainHealth returns health for a specific chain.
func (c *Client) ChainHealth(ctx context.Context, chainID int64) (*ChainHealth, error) {
	var h ChainHealth
	path := fmt.Sprintf("/chains/%d/health", chainID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ---- Endpoints ----

// ListEndpoints returns endpoints for a project.
func (c *Client) ListEndpoints(ctx context.Context, projectID string) ([]Endpoint, error) {
	q := url.Values{"project_id": {projectID}}
	var list EndpointList
	if err := c.do(ctx, http.MethodGet, "/endpoints", q, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// EndpointHealth returns health for a specific endpoint.
func (c *Client) EndpointHealth(ctx context.Context, id string) (*EndpointHealth, error) {
	var h EndpointHealth
	if err := c.do(ctx, http.MethodGet, "/endpoints/"+id+"/health", nil, nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ---- Oracles ----

// OracleHealth returns health for a price feed address.
func (c *Client) OracleHealth(ctx context.Context, feed string) (*OracleHealth, error) {
	var h OracleHealth
	if err := c.do(ctx, http.MethodGet, "/oracles/"+url.PathEscape(feed)+"/health", nil, nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// ---- Runs ----

// ListRuns returns agent runs with optional filters.
func (c *Client) ListRuns(ctx context.Context, params RunListParams) ([]RunSummary, error) {
	q := url.Values{}
	if params.AgentID != "" {
		q.Set("agent_id", params.AgentID)
	}
	if params.ProjectID != "" {
		q.Set("project_id", params.ProjectID)
	}
	if params.Cursor != "" {
		q.Set("cursor", params.Cursor)
	}
	var list RunList
	if err := c.do(ctx, http.MethodGet, "/runs", q, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// GetRun returns a full run with spans.
func (c *Client) GetRun(ctx context.Context, id string) (*RunDetail, error) {
	var r RunDetail
	if err := c.do(ctx, http.MethodGet, "/runs/"+id, nil, nil, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ---- Incidents ----

// ListIncidents returns incidents with optional filters.
func (c *Client) ListIncidents(ctx context.Context, params IncidentListParams) ([]Incident, error) {
	q := url.Values{}
	if params.ProjectID != "" {
		q.Set("project_id", params.ProjectID)
	}
	if params.Status != "" {
		q.Set("status", params.Status)
	}
	var list IncidentList
	if err := c.do(ctx, http.MethodGet, "/incidents", q, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// ---- Policies ----

// ListPolicies returns policies for a project.
func (c *Client) ListPolicies(ctx context.Context, projectID string) ([]Policy, error) {
	q := url.Values{"project_id": {projectID}}
	var list PolicyList
	if err := c.do(ctx, http.MethodGet, "/policies", q, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}

// ---- Usage ----

// GetUsage returns usage records for an org.
func (c *Client) GetUsage(ctx context.Context, params UsageParams) ([]UsageRecord, error) {
	q := url.Values{
		"org_id": {params.OrgID},
		"from":   {params.From},
		"to":     {params.To},
	}
	var list UsageList
	if err := c.do(ctx, http.MethodGet, "/usage", q, nil, &list); err != nil {
		return nil, err
	}
	return list.Data, nil
}
