package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/uwezukwechibuzor/vigil/services/api/internal/auth"
	"github.com/uwezukwechibuzor/vigil/services/internal/db/postgres"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles signup, login and logout.
type AuthHandler struct {
	db     *postgres.Pool
	secure bool // true when running behind HTTPS
}

// NewAuthHandler creates an AuthHandler. secure should be true in production.
func NewAuthHandler(db *postgres.Pool, secure bool) *AuthHandler {
	return &AuthHandler{db: db, secure: secure}
}

type signupRequest struct {
	Name     string `json:"name"`
	OrgName  string `json:"org_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type signupResponse struct {
	OrgID  string `json:"org_id"`
	APIKey string `json:"api_key"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	OrgID string `json:"org_id"`
}

// Signup creates a user, an org, and an initial API key.
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.OrgName = strings.TrimSpace(req.OrgName)

	if req.Email == "" || req.Password == "" || req.OrgName == "" {
		writeAuthError(w, http.StatusBadRequest, "email, password and org_name are required")
		return
	}
	if len(req.Password) < 8 {
		writeAuthError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "internal error")
		return
	}

	displayName := req.Name
	if displayName == "" {
		displayName = req.Email
	}

	user, err := h.db.CreateUser(r.Context(), req.Email, string(hash), displayName)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			writeAuthError(w, http.StatusConflict, "email already registered")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	slug := auth.OrgSlug(req.OrgName)
	org, err := h.db.CreateOrg(r.Context(), slug, req.OrgName, "free")
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "could not create organisation")
		return
	}

	if _, err := h.db.CreateMembership(r.Context(), org.ID, user.ID, "owner"); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "could not create membership")
		return
	}

	plaintext, _, err := h.db.GenerateAPIKey(r.Context(), org.ID, nil, "default", []string{"read", "write"}, nil)
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "could not generate API key")
		return
	}

	auth.SetSessionCookie(w, org.ID.String(), h.secure)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(signupResponse{
		OrgID:  org.ID.String(),
		APIKey: plaintext,
	})
}

// Login authenticates a user and sets a session cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Email == "" || req.Password == "" {
		writeAuthError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Don't leak whether the email exists
		writeAuthError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			writeAuthError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeAuthError(w, http.StatusInternalServerError, "internal error")
		return
	}

	mem, err := h.db.GetMembership(r.Context(), user.ID)
	if err != nil {
		writeAuthError(w, http.StatusUnauthorized, "no organisation found for this user")
		return
	}

	auth.SetSessionCookie(w, mem.OrgID.String(), h.secure)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(loginResponse{OrgID: mem.OrgID.String()})
}

// Logout clears the session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func writeAuthError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"message": msg},
	})
}
