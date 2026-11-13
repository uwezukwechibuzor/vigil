// Package store handles persistence for flight check results.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/uwezukwechibuzor/vigil/services/flightcheck/internal/check"
	dbch "github.com/uwezukwechibuzor/vigil/services/internal/db/clickhouse"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// Store persists flight check results and updates usage counters.
type Store struct {
	ch     *dbch.Client
	pg     *dbpg.Pool
	logger *slog.Logger
}

// New creates a Store. ch and pg may be nil during testing.
func New(ch *dbch.Client, pg *dbpg.Pool, logger *slog.Logger) *Store {
	return &Store{ch: ch, pg: pg, logger: logger}
}

// Save persists the flight check response to ClickHouse.
// If orgID is set it also increments the usage counter.
func (s *Store) Save(ctx context.Context, req *check.Request, resp *check.Response) {
	if s.ch != nil {
		checksJSON, err := json.Marshal(resp.Checks)
		if err != nil {
			s.logger.Error("marshal checks for clickhouse", "error", err)
		} else {
			row := dbch.FlightCheckRow{
				ID:        resp.CheckID,
				ProjectID: req.ProjectID,
				Chain:     fmt.Sprintf("%d", req.ChainID),
				Verdict:   string(resp.Verdict),
				Score:     float32(resp.RiskScore),
				Checks:    string(checksJSON),
				LatencyMS: uint32(resp.LatencyMS), //nolint:gosec
			}
			if err := s.ch.InsertFlightCheck(ctx, row); err != nil {
				s.logger.Error("clickhouse insert flight check", "error", err)
			}
		}
	}

	// Increment usage counter by org if project_id is known.
	if s.pg != nil && req.ProjectID != "" {
		orgID, err := resolveOrgID(ctx, s.pg, req.ProjectID)
		if err != nil {
			s.logger.Warn("resolve org id for usage counter", "error", err)
			return
		}
		if err := s.pg.IncrementFlightChecks(ctx, orgID); err != nil {
			s.logger.Error("increment flight checks counter", "error", err)
		}
	}
}

// resolveOrgID looks up the org_id for a project_id.
func resolveOrgID(ctx context.Context, pg *dbpg.Pool, projectIDStr string) (uuid.UUID, error) {
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse project_id: %w", err)
	}
	var orgID uuid.UUID
	err = pg.QueryRow(ctx,
		`SELECT org_id FROM projects WHERE id = $1`, projectID).Scan(&orgID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("lookup org for project: %w", err)
	}
	return orgID, nil
}
