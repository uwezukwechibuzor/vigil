// Package handler implements the generated StrictServerInterface for all v1 API endpoints.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/uwezukwechibuzor/vigil/services/api/gen"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/analytics"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/auth"
	"github.com/uwezukwechibuzor/vigil/services/api/internal/metering"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// DB aggregates all Postgres operations used by handlers.
type DB interface {
	ListChains(ctx context.Context) ([]postgres.Chain, error)
	GetChainByEVMID(ctx context.Context, chainID int64) (*postgres.Chain, error)
	ListEndpoints(ctx context.Context, projectID uuid.UUID) ([]postgres.Endpoint, error)
	CreateEndpoint(ctx context.Context, projectID, chainID *uuid.UUID, kind postgres.EndpointKind, name, url string) (*postgres.Endpoint, error)
	UpdateEndpoint(ctx context.Context, id uuid.UUID, name, url *string, enabled *bool) (*postgres.Endpoint, error)
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error
	GetEndpoint(ctx context.Context, id uuid.UUID) (*postgres.Endpoint, error)
	ListIncidents(ctx context.Context, projectID uuid.UUID, status *string) ([]postgres.Incident, error)
	GetIncident(ctx context.Context, id uuid.UUID) (*postgres.Incident, error)
	ListPolicies(ctx context.Context, projectID uuid.UUID) ([]postgres.Policy, error)
	CreatePolicy(ctx context.Context, projectID uuid.UUID, name string, rules postgres.PolicyRules, enabled bool) (*postgres.Policy, error)
	GetPolicy(ctx context.Context, id uuid.UUID) (*postgres.Policy, error)
	UpdatePolicy(ctx context.Context, id uuid.UUID, name *string, rules *postgres.PolicyRules, enabled *bool) (*postgres.Policy, error)
	DeletePolicy(ctx context.Context, id uuid.UUID) error
	ListAlertRules(ctx context.Context, projectID uuid.UUID) ([]postgres.AlertRule, error)
	CreateAlertRule(ctx context.Context, projectID uuid.UUID, name, description, severity string, condition map[string]any, enabled bool) (*postgres.AlertRule, error)
	GetAlertRule(ctx context.Context, id uuid.UUID) (*postgres.AlertRule, error)
	UpdateAlertRule(ctx context.Context, id uuid.UUID, name, description, severity *string, condition map[string]any, enabled *bool) (*postgres.AlertRule, error)
	DeleteAlertRule(ctx context.Context, id uuid.UUID) error
	ListAlertChannels(ctx context.Context, orgID uuid.UUID) ([]postgres.AlertChannel, error)
	CreateAlertChannel(ctx context.Context, orgID uuid.UUID, kind, name string, config map[string]any) (*postgres.AlertChannel, error)
	GetAlertChannel(ctx context.Context, id uuid.UUID) (*postgres.AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, id uuid.UUID, name *string, config map[string]any) (*postgres.AlertChannel, error)
	DeleteAlertChannel(ctx context.Context, id uuid.UUID) error
	ListAPIKeys(ctx context.Context, orgID uuid.UUID) ([]postgres.APIKey, error)
	GenerateAPIKey(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, name string, scopes []string, expiresAt *time.Time) (string, *postgres.APIKey, error)
	RevokeAPIKey(ctx context.Context, id uuid.UUID) error
	GetUsageByOrgAndDateRange(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]postgres.UsageCounter, error)
	InsertAuditLog(ctx context.Context, entry postgres.AuditEntry) error
}

// CH aggregates ClickHouse operations used by handlers.
type CH interface {
	GetFlightCheck(ctx context.Context, id string) (*clickhouse.FlightCheckRow, error)
	GetChainHealth(ctx context.Context, chainID string) (*clickhouse.ChainHealthSummary, error)
	GetEndpointHealth(ctx context.Context, endpointID string) (*clickhouse.EndpointHealthSummary, error)
	GetOracleHealth(ctx context.Context, feed string) (*clickhouse.OracleHealthSummary, error)
	ListRuns(ctx context.Context, agentID, projectID, cursor string, limit int) ([]clickhouse.RunSummary, error)
	GetRun(ctx context.Context, id string) (*clickhouse.RunDetail, error)
}

// Handler implements gen.StrictServerInterface.
type Handler struct {
	db             DB
	ch             CH
	meter          *metering.Meter
	analytics      *analytics.Client
	logger         *slog.Logger
	flightcheckURL string
}

// New constructs a Handler. db, ch, and ac may be nil (graceful degradation).
func New(db DB, ch CH, meter *metering.Meter, logger *slog.Logger, ac *analytics.Client) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		db:             db,
		ch:             ch,
		meter:          meter,
		analytics:      ac,
		logger:         logger,
		flightcheckURL: os.Getenv("FLIGHTCHECK_URL"),
	}
}

// --- helpers ---

func apiErr(code, msg string) gen.Error {
	return gen.Error{Error: struct {
		Code    string                  "json:\"code\""
		Details *map[string]interface{} "json:\"details,omitempty\""
		Message string                  "json:\"message\""
	}{Code: code, Message: msg}}
}

func errDBUnavailable() error {
	return fmt.Errorf("database unavailable")
}

func uuidPtr(u uuid.UUID) *openapi_types.UUID {
	v := u
	return &v
}

func (h *Handler) asyncAudit(_ context.Context, orgID uuid.UUID, action, resource, resourceID string) {
	if h.db == nil {
		return
	}
	go func() { // #nosec G118 -- intentionally outlives the request context; uses its own timeout
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.db.InsertAuditLog(rctx, postgres.AuditEntry{
			OrgID:      orgID,
			Action:     action,
			Resource:   resource,
			ResourceID: resourceID,
		})
	}()
}

func (h *Handler) asyncMeter(ctx context.Context, orgID uuid.UUID) {
	if h.meter != nil {
		h.meter.IncrementAPICalls(ctx, orgID)
	}
}

func endpointToGen(e postgres.Endpoint) gen.Endpoint {
	ep := gen.Endpoint{
		Id:        e.ID,
		ProjectId: e.ProjectID,
		Kind:      gen.EndpointKind(e.Kind),
		Name:      e.Name,
		Url:       e.URL,
		Enabled:   e.Enabled,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
	if e.ChainID != nil {
		ep.ChainId = uuidPtr(*e.ChainID)
	}
	return ep
}

func policyToGen(p postgres.Policy) gen.Policy {
	rules := map[string]any{
		"allowed_chains":   p.Rules.AllowedChains,
		"denied_contracts": p.Rules.DeniedContracts,
		"max_value_wei":    p.Rules.MaxValueWei,
	}
	return gen.Policy{
		Id:        p.ID,
		ProjectId: p.ProjectID,
		Name:      p.Name,
		Version:   p.Version,
		Rules:     rules,
		Enabled:   p.Enabled,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

func alertRuleToGen(r postgres.AlertRule) gen.AlertRule {
	ar := gen.AlertRule{
		Id:        r.ID,
		ProjectId: r.ProjectID,
		Name:      r.Name,
		Severity:  gen.AlertRuleSeverity(r.Severity),
		Condition: r.Condition,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
	if r.Description != "" {
		ar.Description = &r.Description
	}
	return ar
}

func alertChannelToGen(ch postgres.AlertChannel) gen.AlertChannel {
	return gen.AlertChannel{
		Id:        ch.ID,
		OrgId:     ch.OrgID,
		Kind:      gen.AlertChannelKind(ch.Kind),
		Name:      ch.Name,
		Config:    ch.Config,
		CreatedAt: ch.CreatedAt,
	}
}

// --- FlightChecks ---

// CreateFlightCheck proxies the request to the flightcheck service.
func (h *Handler) CreateFlightCheck(ctx context.Context, request gen.CreateFlightCheckRequestObject) (gen.CreateFlightCheckResponseObject, error) {
	if h.flightcheckURL == "" {
		return gen.CreateFlightCheck503JSONResponse{
			ServiceUnavailableJSONResponse: gen.ServiceUnavailableJSONResponse(
				apiErr("service_unavailable", "flight check service not configured"),
			),
		}, nil
	}

	body, err := json.Marshal(request.Body)
	if err != nil {
		return nil, fmt.Errorf("marshal flight check request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.flightcheckURL+"/v1/flight-check", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build flight check request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return gen.CreateFlightCheck503JSONResponse{
			ServiceUnavailableJSONResponse: gen.ServiceUnavailableJSONResponse(
				apiErr("service_unavailable", "flight check service unreachable"),
			),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	var result gen.FlightCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode flight check response: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.CreateFlightCheck200JSONResponse(result), nil
}

// GetFlightCheck retrieves a flight check result from ClickHouse.
func (h *Handler) GetFlightCheck(ctx context.Context, request gen.GetFlightCheckRequestObject) (gen.GetFlightCheckResponseObject, error) {
	if h.ch == nil {
		return gen.GetFlightCheck404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "flight check not found")),
		}, nil
	}

	row, err := h.ch.GetFlightCheck(ctx, request.Id)
	if err != nil {
		return gen.GetFlightCheck404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "flight check not found")),
		}, nil
	}

	var checks map[string]any
	if err := json.Unmarshal([]byte(row.Checks), &checks); err != nil {
		checks = map[string]any{"raw": row.Checks}
	}

	verdict := gen.FlightCheckResultVerdict(row.Verdict)
	latencyMs := int(row.LatencyMS)
	result := gen.FlightCheckResult{
		Id:        row.ID,
		ProjectId: &row.ProjectID,
		Chain:     &row.Chain,
		Verdict:   verdict,
		Score:     row.Score,
		Checks:    checks,
		LatencyMs: &latencyMs,
		CreatedAt: time.Now(),
	}
	return gen.GetFlightCheck200JSONResponse(result), nil
}

// --- Chains ---

// ListChains returns all chains with cursor pagination.
func (h *Handler) ListChains(ctx context.Context, _ gen.ListChainsRequestObject) (gen.ListChainsResponseObject, error) {
	if h.db == nil {
		total := int64(0)
		return gen.ListChains200JSONResponse(gen.ChainList{
			Data: []gen.Chain{},
			Meta: gen.ListMeta{Total: &total},
		}), nil
	}

	chains, err := h.db.ListChains(ctx)
	if err != nil {
		h.logger.Error("list chains", "error", err)
		total := int64(0)
		return gen.ListChains200JSONResponse(gen.ChainList{Data: []gen.Chain{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	out := make([]gen.Chain, 0, len(chains))
	for _, c := range chains {
		out = append(out, gen.Chain{
			Id:          c.ID,
			ChainId:     c.ChainID,
			Name:        c.Name,
			NativeToken: c.NativeToken,
			BlockTimeMs: c.BlockTimeMs,
			Enabled:     c.Enabled,
			CreatedAt:   c.CreatedAt,
		})
	}

	total := int64(len(out))
	return gen.ListChains200JSONResponse(gen.ChainList{
		Data: out,
		Meta: gen.ListMeta{Total: &total},
	}), nil
}

// GetChainHealth returns health metrics for a chain.
func (h *Handler) GetChainHealth(ctx context.Context, request gen.GetChainHealthRequestObject) (gen.GetChainHealthResponseObject, error) {
	if h.ch == nil {
		return gen.GetChainHealth200JSONResponse(gen.ChainHealth{
			Status:         gen.ChainHealthStatusHealthy,
			HeadLagBlocks:  0,
			Reorgs24h:      0,
			FeeLevel:       gen.ChainHealthFeeLevelNormal,
			WorstEndpoints: []string{},
		}), nil
	}

	summary, err := h.ch.GetChainHealth(ctx, strconv.FormatInt(request.ChainId, 10))
	if err != nil {
		return gen.GetChainHealth404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "chain not found")),
		}, nil
	}

	status := gen.ChainHealthStatus(summary.Status)
	feeLevel := gen.ChainHealthFeeLevel(summary.FeeLevel)
	return gen.GetChainHealth200JSONResponse(gen.ChainHealth{
		Status:         status,
		HeadLagBlocks:  summary.HeadLagBlocks,
		Reorgs24h:      summary.Reorgs24h,
		FeeLevel:       feeLevel,
		WorstEndpoints: summary.WorstEndpoints,
	}), nil
}

// --- Endpoints ---

// ListEndpoints returns endpoints, filtered by project_id if provided.
func (h *Handler) ListEndpoints(ctx context.Context, request gen.ListEndpointsRequestObject) (gen.ListEndpointsResponseObject, error) {
	if h.db == nil {
		total := int64(0)
		return gen.ListEndpoints200JSONResponse(gen.EndpointList{Data: []gen.Endpoint{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	var projectID uuid.UUID
	if request.Params.ProjectId != nil {
		projectID = *request.Params.ProjectId
	}

	eps, err := h.db.ListEndpoints(ctx, projectID)
	if err != nil {
		h.logger.Error("list endpoints", "error", err)
		total := int64(0)
		return gen.ListEndpoints200JSONResponse(gen.EndpointList{Data: []gen.Endpoint{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	out := make([]gen.Endpoint, 0, len(eps))
	for _, e := range eps {
		out = append(out, endpointToGen(e))
	}

	total := int64(len(out))
	return gen.ListEndpoints200JSONResponse(gen.EndpointList{
		Data: out,
		Meta: gen.ListMeta{Total: &total},
	}), nil
}

// CreateEndpoint creates a new endpoint.
func (h *Handler) CreateEndpoint(ctx context.Context, request gen.CreateEndpointRequestObject) (gen.CreateEndpointResponseObject, error) {
	if h.db == nil {
		return nil, errDBUnavailable()
	}

	projectID := request.Body.ProjectId
	var chainID *uuid.UUID
	if request.Body.ChainId != nil {
		cid := *request.Body.ChainId
		chainID = &cid
	}

	ep, err := h.db.CreateEndpoint(ctx, &projectID, chainID,
		postgres.EndpointKind(request.Body.Kind), request.Body.Name, request.Body.Url)
	if err != nil {
		return nil, fmt.Errorf("create endpoint: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "create", "endpoint", ep.ID.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.CreateEndpoint201JSONResponse(endpointToGen(*ep)), nil
}

// UpdateEndpoint updates endpoint fields.
func (h *Handler) UpdateEndpoint(ctx context.Context, request gen.UpdateEndpointRequestObject) (gen.UpdateEndpointResponseObject, error) {
	if h.db == nil {
		return gen.UpdateEndpoint404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "endpoint not found")),
		}, nil
	}

	id := request.Id
	ep, err := h.db.UpdateEndpoint(ctx, id, request.Body.Name, request.Body.Url, request.Body.Enabled)
	if err != nil {
		return gen.UpdateEndpoint404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "endpoint not found")),
		}, nil
	}
	return gen.UpdateEndpoint200JSONResponse(endpointToGen(*ep)), nil
}

// DeleteEndpoint removes an endpoint.
func (h *Handler) DeleteEndpoint(ctx context.Context, request gen.DeleteEndpointRequestObject) (gen.DeleteEndpointResponseObject, error) {
	if h.db == nil {
		return gen.DeleteEndpoint404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "endpoint not found")),
		}, nil
	}

	id := request.Id
	if err := h.db.DeleteEndpoint(ctx, id); err != nil {
		return gen.DeleteEndpoint404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "endpoint not found")),
		}, nil
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "delete", "endpoint", id.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.DeleteEndpoint204Response{}, nil
}

// GetEndpointHealth returns health metrics for a specific endpoint.
func (h *Handler) GetEndpointHealth(ctx context.Context, request gen.GetEndpointHealthRequestObject) (gen.GetEndpointHealthResponseObject, error) {
	if h.ch == nil {
		return gen.GetEndpointHealth200JSONResponse(gen.EndpointHealth{
			Status:       gen.EndpointHealthStatusHealthy,
			Uptime1hPct:  100,
			LatencyP95Ms: 0,
			ErrorRate:    0,
		}), nil
	}

	summary, err := h.ch.GetEndpointHealth(ctx, request.Id.String())
	if err != nil {
		return gen.GetEndpointHealth404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "endpoint not found")),
		}, nil
	}

	return gen.GetEndpointHealth200JSONResponse(gen.EndpointHealth{
		Status:       gen.EndpointHealthStatus(summary.Status),
		Uptime1hPct:  summary.Uptime1hPct,
		LatencyP95Ms: int(summary.LatencyP95Ms),
		ErrorRate:    summary.ErrorRate,
	}), nil
}

// --- Oracles ---

// GetOracleHealth returns oracle feed health from ClickHouse.
func (h *Handler) GetOracleHealth(ctx context.Context, request gen.GetOracleHealthRequestObject) (gen.GetOracleHealthResponseObject, error) {
	if h.ch == nil {
		return gen.GetOracleHealth404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "oracle feed not found")),
		}, nil
	}

	oh, err := h.ch.GetOracleHealth(ctx, request.Feed)
	if err != nil {
		return gen.GetOracleHealth404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "oracle feed not found")),
		}, nil
	}

	return gen.GetOracleHealth200JSONResponse(gen.OracleHealth{
		Feed:       oh.Feed,
		LastAnswer: oh.LastAnswer,
		UpdatedAt:  oh.UpdatedAt,
		Stale:      oh.Stale,
	}), nil
}

// --- Runs ---

// ListRuns returns a paginated list of agent runs.
func (h *Handler) ListRuns(ctx context.Context, request gen.ListRunsRequestObject) (gen.ListRunsResponseObject, error) {
	if h.ch == nil {
		total := int64(0)
		return gen.ListRuns200JSONResponse(gen.RunList{Data: []gen.RunSummary{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	var agentID, projectID, cursor string
	var limit int
	if request.Params.AgentId != nil {
		agentID = *request.Params.AgentId
	}
	if request.Params.ProjectId != nil {
		projectID = (*request.Params.ProjectId).String()
	}
	if request.Params.Cursor != nil {
		cursor = *request.Params.Cursor
	}
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
	}

	runs, err := h.ch.ListRuns(ctx, agentID, projectID, cursor, limit)
	if err != nil {
		h.logger.Error("list runs", "error", err)
		total := int64(0)
		return gen.ListRuns200JSONResponse(gen.RunList{Data: []gen.RunSummary{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	out := make([]gen.RunSummary, 0, len(runs))
	for _, r := range runs {
		s := gen.RunSummary{
			RunId:        r.RunID,
			AgentId:      r.AgentID,
			TraceId:      r.TraceID,
			SpanCount:    r.SpanCount,
			TotalCostUsd: r.TotalCostUSD,
			CreatedAt:    r.CreatedAt,
		}
		if r.TxHash != "" {
			s.TxHash = &r.TxHash
		}
		out = append(out, s)
	}

	total := int64(len(out))
	return gen.ListRuns200JSONResponse(gen.RunList{
		Data: out,
		Meta: gen.ListMeta{Total: &total},
	}), nil
}

// GetRun returns the full trace detail for one agent run.
func (h *Handler) GetRun(ctx context.Context, request gen.GetRunRequestObject) (gen.GetRunResponseObject, error) {
	if h.ch == nil {
		return gen.GetRun404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "run not found")),
		}, nil
	}

	detail, err := h.ch.GetRun(ctx, request.Id)
	if err != nil {
		return gen.GetRun404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "run not found")),
		}, nil
	}

	spans := make([]gen.Span, 0, len(detail.Spans))
	for _, s := range detail.Spans {
		sp := gen.Span{
			TraceId:     s.TraceID,
			SpanId:      s.SpanID,
			Name:        s.Name,
			Kind:        s.Kind,
			ServiceName: s.ServiceName,
			StartTime:   s.StartTime,
			EndTime:     s.EndTime,
			StatusCode:  s.StatusCode,
		}
		if s.ParentSpanID != "" {
			sp.ParentSpanId = &s.ParentSpanID
		}
		if s.StatusMsg != "" {
			sp.StatusMsg = &s.StatusMsg
		}
		if s.GenAIModel != "" {
			sp.GenAiModel = &s.GenAIModel
		}
		inTokens := int(s.InputTokens)
		outTokens := int(s.OutputTokens)
		if inTokens > 0 {
			sp.InputTokens = &inTokens
		}
		if outTokens > 0 {
			sp.OutputTokens = &outTokens
		}
		if s.LLMCostUSD > 0 {
			sp.LlmCostUsd = &s.LLMCostUSD
		}
		spans = append(spans, sp)
	}

	var txHashPtr *string
	if detail.TxHash != "" {
		txHashPtr = &detail.TxHash
	}
	out := gen.RunDetail{
		RunId:        detail.RunID,
		AgentId:      detail.AgentID,
		TraceId:      detail.TraceID,
		SpanCount:    detail.SpanCount,
		TotalCostUsd: detail.TotalCostUSD,
		CreatedAt:    detail.CreatedAt,
		TxHash:       txHashPtr,
		Spans:        spans,
	}
	return gen.GetRun200JSONResponse(out), nil
}

// --- Incidents ---

// ListIncidents returns a paginated list of incidents.
func (h *Handler) ListIncidents(ctx context.Context, request gen.ListIncidentsRequestObject) (gen.ListIncidentsResponseObject, error) {
	if h.db == nil {
		total := int64(0)
		return gen.ListIncidents200JSONResponse(gen.IncidentList{Data: []gen.Incident{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	var projectID uuid.UUID
	if request.Params.ProjectId != nil {
		projectID = *request.Params.ProjectId
	}

	var statusStr *string
	if request.Params.Status != nil {
		s := string(*request.Params.Status)
		statusStr = &s
	}

	incidents, err := h.db.ListIncidents(ctx, projectID, statusStr)
	if err != nil {
		h.logger.Error("list incidents", "error", err)
		total := int64(0)
		return gen.ListIncidents200JSONResponse(gen.IncidentList{Data: []gen.Incident{}, Meta: gen.ListMeta{Total: &total}}), nil
	}

	out := make([]gen.Incident, 0, len(incidents))
	for _, i := range incidents {
		inc := gen.Incident{
			Id:        i.ID,
			ProjectId: i.ProjectID,
			Status:    gen.IncidentStatus(i.Status),
			Severity:  gen.IncidentSeverity(i.Severity),
			Title:     i.Title,
			StartedAt: i.StartedAt,
			CreatedAt: i.CreatedAt,
		}
		if i.RuleID != nil {
			inc.RuleId = uuidPtr(*i.RuleID)
		}
		inc.AcknowledgedAt = i.AcknowledgedAt
		inc.ResolvedAt = i.ResolvedAt
		out = append(out, inc)
	}

	total := int64(len(out))
	return gen.ListIncidents200JSONResponse(gen.IncidentList{
		Data: out,
		Meta: gen.ListMeta{Total: &total},
	}), nil
}

// GetIncident returns a single incident.
func (h *Handler) GetIncident(ctx context.Context, request gen.GetIncidentRequestObject) (gen.GetIncidentResponseObject, error) {
	if h.db == nil {
		return gen.GetIncident404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "incident not found")),
		}, nil
	}

	id := request.Id
	i, err := h.db.GetIncident(ctx, id)
	if err != nil {
		return gen.GetIncident404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "incident not found")),
		}, nil
	}

	inc := gen.Incident{
		Id:        i.ID,
		ProjectId: i.ProjectID,
		Status:    gen.IncidentStatus(i.Status),
		Severity:  gen.IncidentSeverity(i.Severity),
		Title:     i.Title,
		StartedAt: i.StartedAt,
		CreatedAt: i.CreatedAt,
	}
	if i.RuleID != nil {
		inc.RuleId = uuidPtr(*i.RuleID)
	}
	inc.AcknowledgedAt = i.AcknowledgedAt
	inc.ResolvedAt = i.ResolvedAt
	return gen.GetIncident200JSONResponse(inc), nil
}

// --- Policies ---

// ListPolicies returns all policies for a project.
func (h *Handler) ListPolicies(ctx context.Context, request gen.ListPoliciesRequestObject) (gen.ListPoliciesResponseObject, error) {
	if h.db == nil {
		return gen.ListPolicies200JSONResponse(gen.PolicyList{Data: []gen.Policy{}}), nil
	}

	projectID := request.Params.ProjectId
	policies, err := h.db.ListPolicies(ctx, projectID)
	if err != nil {
		h.logger.Error("list policies", "error", err)
		return gen.ListPolicies200JSONResponse(gen.PolicyList{Data: []gen.Policy{}}), nil
	}

	out := make([]gen.Policy, 0, len(policies))
	for _, p := range policies {
		out = append(out, policyToGen(p))
	}
	return gen.ListPolicies200JSONResponse(gen.PolicyList{Data: out}), nil
}

// CreatePolicy creates a new policy.
func (h *Handler) CreatePolicy(ctx context.Context, request gen.CreatePolicyRequestObject) (gen.CreatePolicyResponseObject, error) {
	if h.db == nil {
		return nil, errDBUnavailable()
	}

	projectID := request.Body.ProjectId
	enabled := true
	if request.Body.Enabled != nil {
		enabled = *request.Body.Enabled
	}

	rulesJSON, _ := json.Marshal(request.Body.Rules)
	var rules postgres.PolicyRules
	_ = json.Unmarshal(rulesJSON, &rules)

	pol, err := h.db.CreatePolicy(ctx, projectID, request.Body.Name, rules, enabled)
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "create", "policy", pol.ID.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.CreatePolicy201JSONResponse(policyToGen(*pol)), nil
}

// GetPolicy returns a single policy.
func (h *Handler) GetPolicy(ctx context.Context, request gen.GetPolicyRequestObject) (gen.GetPolicyResponseObject, error) {
	if h.db == nil {
		return gen.GetPolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}

	id := request.Id
	pol, err := h.db.GetPolicy(ctx, id)
	if err != nil {
		return gen.GetPolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}
	return gen.GetPolicy200JSONResponse(policyToGen(*pol)), nil
}

// UpdatePolicy applies partial updates to a policy.
func (h *Handler) UpdatePolicy(ctx context.Context, request gen.UpdatePolicyRequestObject) (gen.UpdatePolicyResponseObject, error) {
	if h.db == nil {
		return gen.UpdatePolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}

	id := request.Id

	var rules *postgres.PolicyRules
	if request.Body.Rules != nil {
		rulesJSON, _ := json.Marshal(request.Body.Rules)
		var r postgres.PolicyRules
		if err := json.Unmarshal(rulesJSON, &r); err == nil {
			rules = &r
		}
	}

	pol, err := h.db.UpdatePolicy(ctx, id, request.Body.Name, rules, request.Body.Enabled)
	if err != nil {
		return gen.UpdatePolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}
	return gen.UpdatePolicy200JSONResponse(policyToGen(*pol)), nil
}

// DeletePolicy removes a policy.
func (h *Handler) DeletePolicy(ctx context.Context, request gen.DeletePolicyRequestObject) (gen.DeletePolicyResponseObject, error) {
	if h.db == nil {
		return gen.DeletePolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}

	id := request.Id
	if err := h.db.DeletePolicy(ctx, id); err != nil {
		return gen.DeletePolicy404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "policy not found")),
		}, nil
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "delete", "policy", id.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.DeletePolicy204Response{}, nil
}

// --- Alert Rules ---

// ListAlertRules returns alert rules for a project.
func (h *Handler) ListAlertRules(ctx context.Context, request gen.ListAlertRulesRequestObject) (gen.ListAlertRulesResponseObject, error) {
	if h.db == nil {
		return gen.ListAlertRules200JSONResponse(gen.AlertRuleList{Data: []gen.AlertRule{}}), nil
	}

	projectID := request.Params.ProjectId
	rules, err := h.db.ListAlertRules(ctx, projectID)
	if err != nil {
		h.logger.Error("list alert rules", "error", err)
		return gen.ListAlertRules200JSONResponse(gen.AlertRuleList{Data: []gen.AlertRule{}}), nil
	}

	out := make([]gen.AlertRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, alertRuleToGen(r))
	}
	return gen.ListAlertRules200JSONResponse(gen.AlertRuleList{Data: out}), nil
}

// CreateAlertRule creates a new alert rule.
func (h *Handler) CreateAlertRule(ctx context.Context, request gen.CreateAlertRuleRequestObject) (gen.CreateAlertRuleResponseObject, error) {
	if h.db == nil {
		return nil, errDBUnavailable()
	}

	projectID := request.Body.ProjectId
	desc := ""
	if request.Body.Description != nil {
		desc = *request.Body.Description
	}
	enabled := true
	if request.Body.Enabled != nil {
		enabled = *request.Body.Enabled
	}

	r, err := h.db.CreateAlertRule(ctx, projectID, request.Body.Name, desc,
		string(request.Body.Severity), request.Body.Condition, enabled)
	if err != nil {
		return nil, fmt.Errorf("create alert rule: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "create", "alert_rule", r.ID.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.CreateAlertRule201JSONResponse(alertRuleToGen(*r)), nil
}

// GetAlertRule returns a single alert rule.
func (h *Handler) GetAlertRule(ctx context.Context, request gen.GetAlertRuleRequestObject) (gen.GetAlertRuleResponseObject, error) {
	if h.db == nil {
		return gen.GetAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}

	id := request.Id
	r, err := h.db.GetAlertRule(ctx, id)
	if err != nil {
		return gen.GetAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}
	return gen.GetAlertRule200JSONResponse(alertRuleToGen(*r)), nil
}

// UpdateAlertRule applies partial updates to an alert rule.
func (h *Handler) UpdateAlertRule(ctx context.Context, request gen.UpdateAlertRuleRequestObject) (gen.UpdateAlertRuleResponseObject, error) {
	if h.db == nil {
		return gen.UpdateAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}

	id := request.Id

	var severityStr *string
	if request.Body.Severity != nil {
		s := string(*request.Body.Severity)
		severityStr = &s
	}

	var cond map[string]any
	if request.Body.Condition != nil {
		cond = *request.Body.Condition
	}
	r, err := h.db.UpdateAlertRule(ctx, id, request.Body.Name, request.Body.Description,
		severityStr, cond, request.Body.Enabled)
	if err != nil {
		return gen.UpdateAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}
	return gen.UpdateAlertRule200JSONResponse(alertRuleToGen(*r)), nil
}

// DeleteAlertRule removes an alert rule.
func (h *Handler) DeleteAlertRule(ctx context.Context, request gen.DeleteAlertRuleRequestObject) (gen.DeleteAlertRuleResponseObject, error) {
	if h.db == nil {
		return gen.DeleteAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}

	id := request.Id
	if err := h.db.DeleteAlertRule(ctx, id); err != nil {
		return gen.DeleteAlertRule404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert rule not found")),
		}, nil
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "delete", "alert_rule", id.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.DeleteAlertRule204Response{}, nil
}

// --- Alert Channels ---

// ListAlertChannels returns alert channels for an org.
func (h *Handler) ListAlertChannels(ctx context.Context, request gen.ListAlertChannelsRequestObject) (gen.ListAlertChannelsResponseObject, error) {
	if h.db == nil {
		return gen.ListAlertChannels200JSONResponse(gen.AlertChannelList{Data: []gen.AlertChannel{}}), nil
	}

	orgID := request.Params.OrgId
	channels, err := h.db.ListAlertChannels(ctx, orgID)
	if err != nil {
		h.logger.Error("list alert channels", "error", err)
		return gen.ListAlertChannels200JSONResponse(gen.AlertChannelList{Data: []gen.AlertChannel{}}), nil
	}

	out := make([]gen.AlertChannel, 0, len(channels))
	for _, ch := range channels {
		out = append(out, alertChannelToGen(ch))
	}
	return gen.ListAlertChannels200JSONResponse(gen.AlertChannelList{Data: out}), nil
}

// CreateAlertChannel creates a new alert channel.
func (h *Handler) CreateAlertChannel(ctx context.Context, request gen.CreateAlertChannelRequestObject) (gen.CreateAlertChannelResponseObject, error) {
	if h.db == nil {
		return nil, errDBUnavailable()
	}

	orgID := request.Body.OrgId
	ch, err := h.db.CreateAlertChannel(ctx, orgID, string(request.Body.Kind), request.Body.Name, request.Body.Config)
	if err != nil {
		return nil, fmt.Errorf("create alert channel: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "create", "alert_channel", ch.ID.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.CreateAlertChannel201JSONResponse(alertChannelToGen(*ch)), nil
}

// GetAlertChannel returns a single alert channel.
func (h *Handler) GetAlertChannel(ctx context.Context, request gen.GetAlertChannelRequestObject) (gen.GetAlertChannelResponseObject, error) {
	if h.db == nil {
		return gen.GetAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}

	id := request.Id
	ch, err := h.db.GetAlertChannel(ctx, id)
	if err != nil {
		return gen.GetAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}
	return gen.GetAlertChannel200JSONResponse(alertChannelToGen(*ch)), nil
}

// UpdateAlertChannel applies partial updates to an alert channel.
func (h *Handler) UpdateAlertChannel(ctx context.Context, request gen.UpdateAlertChannelRequestObject) (gen.UpdateAlertChannelResponseObject, error) {
	if h.db == nil {
		return gen.UpdateAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}

	id := request.Id
	var cfg map[string]any
	if request.Body.Config != nil {
		cfg = *request.Body.Config
	}
	ch, err := h.db.UpdateAlertChannel(ctx, id, request.Body.Name, cfg)
	if err != nil {
		return gen.UpdateAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}
	return gen.UpdateAlertChannel200JSONResponse(alertChannelToGen(*ch)), nil
}

// DeleteAlertChannel removes an alert channel.
func (h *Handler) DeleteAlertChannel(ctx context.Context, request gen.DeleteAlertChannelRequestObject) (gen.DeleteAlertChannelResponseObject, error) {
	if h.db == nil {
		return gen.DeleteAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}

	id := request.Id
	if err := h.db.DeleteAlertChannel(ctx, id); err != nil {
		return gen.DeleteAlertChannel404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "alert channel not found")),
		}, nil
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "delete", "alert_channel", id.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.DeleteAlertChannel204Response{}, nil
}

// --- API Keys ---

// ListAPIKeys returns all API keys for an org.
func (h *Handler) ListAPIKeys(ctx context.Context, request gen.ListAPIKeysRequestObject) (gen.ListAPIKeysResponseObject, error) {
	if h.db == nil {
		return gen.ListAPIKeys200JSONResponse(gen.APIKeyList{Data: []gen.APIKey{}}), nil
	}

	orgID := request.Params.OrgId
	keys, err := h.db.ListAPIKeys(ctx, orgID)
	if err != nil {
		h.logger.Error("list api keys", "error", err)
		return gen.ListAPIKeys200JSONResponse(gen.APIKeyList{Data: []gen.APIKey{}}), nil
	}

	out := make([]gen.APIKey, 0, len(keys))
	for _, k := range keys {
		ak := gen.APIKey{
			Id:        k.ID,
			OrgId:     k.OrgID,
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Scopes:    k.Scopes,
			CreatedAt: k.CreatedAt,
		}
		if k.ProjectID != nil {
			ak.ProjectId = uuidPtr(*k.ProjectID)
		}
		ak.ExpiresAt = k.ExpiresAt
		ak.LastUsedAt = k.LastUsedAt
		ak.RevokedAt = k.RevokedAt
		out = append(out, ak)
	}
	return gen.ListAPIKeys200JSONResponse(gen.APIKeyList{Data: out}), nil
}

// CreateAPIKey creates a new API key and returns the plaintext once.
func (h *Handler) CreateAPIKey(ctx context.Context, request gen.CreateAPIKeyRequestObject) (gen.CreateAPIKeyResponseObject, error) {
	if h.db == nil {
		return nil, errDBUnavailable()
	}

	orgID := request.Body.OrgId
	var projectID *uuid.UUID
	if request.Body.ProjectId != nil {
		pid := *request.Body.ProjectId
		projectID = &pid
	}

	plaintext, k, err := h.db.GenerateAPIKey(ctx, orgID, projectID, request.Body.Name, request.Body.Scopes, request.Body.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("generate api key: %w", err)
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "create", "api_key", k.ID.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	out := gen.APIKeyWithPlaintext{
		Id:        k.ID,
		OrgId:     k.OrgID,
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		Scopes:    k.Scopes,
		Key:       plaintext,
		CreatedAt: k.CreatedAt,
	}
	if k.ProjectID != nil {
		out.ProjectId = uuidPtr(*k.ProjectID)
	}
	out.ExpiresAt = k.ExpiresAt
	return gen.CreateAPIKey201JSONResponse(out), nil
}

// DeleteAPIKey revokes an API key.
func (h *Handler) DeleteAPIKey(ctx context.Context, request gen.DeleteAPIKeyRequestObject) (gen.DeleteAPIKeyResponseObject, error) {
	if h.db == nil {
		return gen.DeleteAPIKey404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "api key not found")),
		}, nil
	}

	id := request.Id
	if err := h.db.RevokeAPIKey(ctx, id); err != nil {
		return gen.DeleteAPIKey404JSONResponse{
			NotFoundJSONResponse: gen.NotFoundJSONResponse(apiErr("not_found", "api key not found")),
		}, nil
	}

	key := auth.APIKeyFromContext(ctx)
	if key != nil {
		h.asyncAudit(ctx, key.OrgID, "delete", "api_key", id.String())
		h.asyncMeter(ctx, key.OrgID)
	}

	return gen.DeleteAPIKey204Response{}, nil
}

// --- Usage ---

// GetUsage returns daily usage counters for an org.
func (h *Handler) GetUsage(ctx context.Context, request gen.GetUsageRequestObject) (gen.GetUsageResponseObject, error) {
	if h.db == nil {
		return gen.GetUsage200JSONResponse(gen.UsageList{Data: []gen.UsageCounter{}}), nil
	}

	orgID := request.Params.OrgId
	from := request.Params.From.Time
	to := request.Params.To.Time

	counters, err := h.db.GetUsageByOrgAndDateRange(ctx, orgID, from, to)
	if err != nil {
		h.logger.Error("get usage", "error", err)
		return gen.GetUsage200JSONResponse(gen.UsageList{Data: []gen.UsageCounter{}}), nil
	}

	out := make([]gen.UsageCounter, 0, len(counters))
	for _, c := range counters {
		out = append(out, gen.UsageCounter{
			OrgId:        c.OrgID,
			Day:          openapi_types.Date{Time: c.Day},
			Traces:       c.Traces,
			FlightChecks: c.FlightChecks,
			ApiCalls:     c.APICalls,
		})
	}
	return gen.GetUsage200JSONResponse(gen.UsageList{Data: out}), nil
}

// Verify Handler implements StrictServerInterface at compile time.
var _ gen.StrictServerInterface = (*Handler)(nil)
