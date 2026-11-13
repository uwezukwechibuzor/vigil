// Package evaluator evaluates alert rules and dispatches notifications.
package evaluator

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/uwezukwechibuzor/vigil/services/alerts/internal/channels"
	dbpg "github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
)

// Evaluator evaluates alert rules and dispatches notifications.
type Evaluator struct {
	pg          *dbpg.Pool
	dispatchers map[string]channels.Dispatcher
	logger      *slog.Logger
}

// New creates an Evaluator.
func New(pg *dbpg.Pool, dispatchers []channels.Dispatcher, logger *slog.Logger) *Evaluator {
	m := make(map[string]channels.Dispatcher, len(dispatchers))
	for _, d := range dispatchers {
		m[d.Kind()] = d
	}
	return &Evaluator{pg: pg, dispatchers: m, logger: logger}
}

// HandleIncident dispatches notifications for a newly created incident.
// It looks up alert rules for the project and fires matching dispatchers.
func (e *Evaluator) HandleIncident(ctx context.Context, projectID uuid.UUID, incident *dbpg.Incident) error {
	if e.pg == nil {
		return nil
	}
	rules, err := e.pg.ListAlertRules(ctx, projectID)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		condType, _ := rule.Condition["type"].(string)
		if condType != "anomaly" && condType != "" {
			continue
		}

		n := channels.Notification{
			Title:      incident.Title,
			Body:       incident.Summary,
			Severity:   incident.Severity,
			IncidentID: incident.ID.String(),
		}

		if d, ok := e.dispatchers[rule.Severity]; ok {
			if sendErr := d.Send(ctx, n); sendErr != nil {
				e.logger.Error("dispatch", "kind", rule.Severity, "err", sendErr)
			}
		}
	}
	return nil
}
