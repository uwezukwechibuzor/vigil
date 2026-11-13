// Package api_test provides contract tests verifying the API against the OpenAPI spec.
package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	api "github.com/uwezukwechibuzor/vigil/services/api"
	"github.com/uwezukwechibuzor/vigil/services/api/gen"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/handler"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/metering"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// --- mock DB for contract tests ---

type mockDB struct{}

func (m *mockDB) ListChains(_ context.Context) ([]postgres.Chain, error) {
	return []postgres.Chain{{
		ID:          uuid.New(),
		ChainID:     1,
		Name:        "Ethereum",
		NativeToken: "ETH",
		BlockTimeMs: 12000,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}}, nil
}

func (m *mockDB) GetChainByEVMID(_ context.Context, _ int64) (*postgres.Chain, error) {
	return nil, nil
}

func (m *mockDB) ListEndpoints(_ context.Context, _ uuid.UUID) ([]postgres.Endpoint, error) {
	return []postgres.Endpoint{{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Kind:      postgres.EndpointKindRPC,
		Name:      "test endpoint",
		URL:       "https://rpc.example.com",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}, nil
}

func (m *mockDB) CreateEndpoint(_ context.Context, projectID, _ *uuid.UUID, kind postgres.EndpointKind, name, url string) (*postgres.Endpoint, error) {
	return &postgres.Endpoint{
		ID:        uuid.New(),
		ProjectID: *projectID,
		Kind:      kind,
		Name:      name,
		URL:       url,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockDB) UpdateEndpoint(_ context.Context, id uuid.UUID, _, _ *string, _ *bool) (*postgres.Endpoint, error) {
	return &postgres.Endpoint{
		ID:        id,
		ProjectID: uuid.New(),
		Kind:      postgres.EndpointKindRPC,
		Name:      "updated",
		URL:       "https://rpc.example.com",
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *mockDB) DeleteEndpoint(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockDB) GetEndpoint(_ context.Context, _ uuid.UUID) (*postgres.Endpoint, error) {
	return &postgres.Endpoint{ID: uuid.New(), ProjectID: uuid.New(), Kind: postgres.EndpointKindRPC, Name: "ep", URL: "https://rpc.example.com", Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
}
func (m *mockDB) ListIncidents(_ context.Context, _ uuid.UUID, _ *string) ([]postgres.Incident, error) {
	return nil, nil
}
func (m *mockDB) GetIncident(_ context.Context, _ uuid.UUID) (*postgres.Incident, error) {
	return nil, nil
}
func (m *mockDB) ListPolicies(_ context.Context, _ uuid.UUID) ([]postgres.Policy, error) {
	return nil, nil
}
func (m *mockDB) CreatePolicy(_ context.Context, projectID uuid.UUID, name string, rules postgres.PolicyRules, enabled bool) (*postgres.Policy, error) {
	return &postgres.Policy{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      name,
		Version:   1,
		Rules:     rules,
		Enabled:   enabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
func (m *mockDB) GetPolicy(_ context.Context, _ uuid.UUID) (*postgres.Policy, error) { return nil, nil }
func (m *mockDB) UpdatePolicy(_ context.Context, _ uuid.UUID, _ *string, _ *postgres.PolicyRules, _ *bool) (*postgres.Policy, error) {
	return nil, nil
}
func (m *mockDB) DeletePolicy(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockDB) ListAlertRules(_ context.Context, _ uuid.UUID) ([]postgres.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) CreateAlertRule(_ context.Context, projectID uuid.UUID, name, _, severity string, cond map[string]any, enabled bool) (*postgres.AlertRule, error) {
	return &postgres.AlertRule{
		ID:        uuid.New(),
		ProjectID: projectID,
		Name:      name,
		Severity:  severity,
		Condition: cond,
		Enabled:   enabled,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}
func (m *mockDB) GetAlertRule(_ context.Context, _ uuid.UUID) (*postgres.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) UpdateAlertRule(_ context.Context, _ uuid.UUID, _, _, _ *string, _ map[string]any, _ *bool) (*postgres.AlertRule, error) {
	return nil, nil
}
func (m *mockDB) DeleteAlertRule(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockDB) ListAlertChannels(_ context.Context, _ uuid.UUID) ([]postgres.AlertChannel, error) {
	return nil, nil
}
func (m *mockDB) CreateAlertChannel(_ context.Context, orgID uuid.UUID, kind, name string, cfg map[string]any) (*postgres.AlertChannel, error) {
	return &postgres.AlertChannel{
		ID:        uuid.New(),
		OrgID:     orgID,
		Kind:      kind,
		Name:      name,
		Config:    cfg,
		CreatedAt: time.Now(),
	}, nil
}
func (m *mockDB) GetAlertChannel(_ context.Context, _ uuid.UUID) (*postgres.AlertChannel, error) {
	return nil, nil
}
func (m *mockDB) UpdateAlertChannel(_ context.Context, _ uuid.UUID, _ *string, _ map[string]any) (*postgres.AlertChannel, error) {
	return nil, nil
}
func (m *mockDB) DeleteAlertChannel(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockDB) ListAPIKeys(_ context.Context, _ uuid.UUID) ([]postgres.APIKey, error) {
	now := time.Now()
	return []postgres.APIKey{{
		ID:        uuid.New(),
		OrgID:     uuid.New(),
		Name:      "test-key",
		KeyPrefix: "vgl_test",
		KeyHash:   "fakehash",
		Scopes:    []string{"read"},
		CreatedAt: now,
	}}, nil
}
func (m *mockDB) GenerateAPIKey(_ context.Context, orgID uuid.UUID, projectID *uuid.UUID, name string, scopes []string, _ *time.Time) (string, *postgres.APIKey, error) {
	k := &postgres.APIKey{
		ID:        uuid.New(),
		OrgID:     orgID,
		ProjectID: projectID,
		Name:      name,
		KeyPrefix: "vgl_test",
		KeyHash:   "fakehash",
		Scopes:    scopes,
		CreatedAt: time.Now(),
	}
	return "vgl_testplaintext", k, nil
}
func (m *mockDB) RevokeAPIKey(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockDB) GetUsageByOrgAndDateRange(_ context.Context, orgID uuid.UUID, _, _ time.Time) ([]postgres.UsageCounter, error) {
	return []postgres.UsageCounter{{
		ID:       uuid.New(),
		OrgID:    orgID,
		Day:      time.Now().Truncate(24 * time.Hour),
		Traces:   10,
		APICalls: 100,
	}}, nil
}
func (m *mockDB) IncrementAPICalls(_ context.Context, _ uuid.UUID) error        { return nil }
func (m *mockDB) InsertAuditLog(_ context.Context, _ postgres.AuditEntry) error { return nil }

// mockCH is a no-op ClickHouse mock.
type mockCH struct{}

func (m *mockCH) GetFlightCheck(_ context.Context, _ string) (*clickhouse.FlightCheckRow, error) {
	return nil, nil
}
func (m *mockCH) GetChainHealth(_ context.Context, _ string) (*clickhouse.ChainHealthSummary, error) {
	return &clickhouse.ChainHealthSummary{Status: "healthy", WorstEndpoints: []string{}}, nil
}
func (m *mockCH) GetEndpointHealth(_ context.Context, _ string) (*clickhouse.EndpointHealthSummary, error) {
	return &clickhouse.EndpointHealthSummary{Status: "healthy"}, nil
}
func (m *mockCH) GetOracleHealth(_ context.Context, _ string) (*clickhouse.OracleHealthSummary, error) {
	return nil, nil
}
func (m *mockCH) ListRuns(_ context.Context, _, _, _ string, _ int) ([]clickhouse.RunSummary, error) {
	return nil, nil
}
func (m *mockCH) GetRun(_ context.Context, _ string) (*clickhouse.RunDetail, error) { return nil, nil }

// newTestServer builds an httptest.Server backed by the full handler with mocks.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := &mockDB{}
	ch := &mockCH{}
	meter := metering.New(db, nil)

	h := handler.New(db, ch, meter, nil, nil)
	strictH := gen.NewStrictHandler(h, nil)
	r := chi.NewRouter()
	apiHandler := gen.HandlerFromMux(strictH, chi.NewRouter())
	r.Mount("/v1", apiHandler)
	return httptest.NewServer(r)
}

func TestContract_LoadSpec(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping contract test in short mode")
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(api.OpenAPISpec)
	if err != nil {
		t.Fatalf("failed to load OpenAPI spec: %v", err)
	}
	if doc.Info == nil {
		t.Fatal("spec has no info")
	}
	if doc.Info.Title == "" {
		t.Fatal("spec has no title")
	}
}

func doGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestContract_GetChains(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp := doGet(t, srv.URL+"/v1/chains")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body gen.ChainList
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode chain list: %v", err)
	}
	if len(body.Data) == 0 {
		t.Error("expected at least one chain in response")
	}
}

func TestContract_GetEndpoints(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	orgID := uuid.New()
	resp := doGet(t, srv.URL+"/v1/endpoints?project_id="+orgID.String())
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body gen.EndpointList
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode endpoint list: %v", err)
	}
	if len(body.Data) == 0 {
		t.Error("expected at least one endpoint")
	}
}

func TestContract_GetAPIKeys(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	orgID := uuid.New()
	resp := doGet(t, srv.URL+"/v1/api-keys?org_id="+orgID.String())
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body gen.APIKeyList
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode api key list: %v", err)
	}
	if len(body.Data) == 0 {
		t.Error("expected at least one api key")
	}
}

func TestContract_GetUsage(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	orgID := uuid.New()
	resp := doGet(t, srv.URL+"/v1/usage?org_id="+orgID.String()+"&from=2026-01-01&to=2026-01-31")
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body gen.UsageList
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode usage list: %v", err)
	}
	if len(body.Data) == 0 {
		t.Error("expected at least one usage counter")
	}
	if body.Data[0].ApiCalls == 0 {
		t.Error("expected non-zero api_calls")
	}
}

func TestContract_AllOperationsRespond(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full operation scan in short mode")
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(api.OpenAPISpec)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	srv := newTestServer(t)
	defer srv.Close()

	orgID := uuid.New()
	projectID := uuid.New()

	// Map of spec-relative path patterns to query parameters needed.
	pathQueries := map[string]string{
		"/chains":         "",
		"/endpoints":      "?project_id=" + projectID.String(),
		"/incidents":      "",
		"/policies":       "?project_id=" + projectID.String(),
		"/alert-rules":    "?project_id=" + projectID.String(),
		"/alert-channels": "?org_id=" + orgID.String(),
		"/api-keys":       "?org_id=" + orgID.String(),
		"/usage":          "?org_id=" + orgID.String() + "&from=2026-01-01&to=2026-01-31",
		"/runs":           "",
	}

	for path, item := range doc.Paths.Map() {
		if item.Get == nil {
			continue
		}
		query := pathQueries[path]
		url := srv.URL + "/v1" + path + query
		// Skip paths with path parameters.
		if containsParam(path) {
			continue
		}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		if err != nil {
			t.Errorf("GET %s new request: %v", path, err)
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		_ = resp.Body.Close()
		// Verify response code is documented.
		codeStr := fmt.Sprintf("%d", resp.StatusCode)
		if _, ok := item.Get.Responses.Map()[codeStr]; !ok {
			t.Errorf("GET %s: undocumented response code %d", path, resp.StatusCode)
		}
	}
}

func containsParam(path string) bool {
	for _, c := range path {
		if c == '{' {
			return true
		}
	}
	return false
}

// Ensure openapi_types is used.
var _ openapi_types.UUID
