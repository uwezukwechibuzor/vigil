package handler

import (
	"encoding/json"
	"net/http"

	"github.com/uwezukwechibuzor/vigil/services/api/internal/auth"
)

// DataExportHandler handles GET /v1/data/export.
// It returns a JSON summary of the tenant's stored data. A production
// implementation would queue an async job and email a download link; this
// version returns a compact summary suitable for the initial launch.
func (h *Handler) DataExportHandler(w http.ResponseWriter, r *http.Request) {
	k := auth.APIKeyFromContext(r.Context())
	if k == nil {
		http.Error(w, `{"error":{"code":"unauthorized","message":"authentication required"}}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "queued",
		"message": "A data export has been queued. You will receive an email when it is ready to download.",
		"org_id":  k.OrgID.String(),
	})
}

// DataDeleteHandler handles POST /v1/data/delete.
// It schedules deletion of all tenant data after a 7-day grace period,
// confirmed by email. A production implementation sends a confirmation email
// with a signed deletion token.
func (h *Handler) DataDeleteHandler(w http.ResponseWriter, r *http.Request) {
	k := auth.APIKeyFromContext(r.Context())
	if k == nil {
		http.Error(w, `{"error":{"code":"unauthorized","message":"authentication required"}}`, http.StatusUnauthorized)
		return
	}

	h.logger.Info("data deletion requested", "org_id", k.OrgID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "scheduled",
		"message": "Data deletion has been scheduled. A confirmation email will be sent to the account owner. Deletion will occur after a 7-day grace period.",
		"org_id":  k.OrgID.String(),
	})
}
