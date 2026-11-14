package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const sessionCookieName = "vigil_session"

// sessionPayload is the JSON body of the vigil_session cookie.
type sessionPayload struct {
	OrgID string `json:"org_id"`
}

// SetSessionCookie writes a vigil_session cookie scoped to the given org.
func SetSessionCookie(w http.ResponseWriter, orgID string, secure bool) {
	payload, _ := json.Marshal(sessionPayload{OrgID: orgID})
	encoded := base64.StdEncoding.EncodeToString(payload)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    encoded,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(30 * 24 * time.Hour),
	})
}

// ClearSessionCookie removes the vigil_session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// sessionFromRequest extracts org_id from the vigil_session cookie.
// Returns empty string if not present or malformed.
func sessionFromRequest(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	orgID, err := parseCookieSession(c.Value)
	if err != nil {
		return ""
	}
	return orgID
}

// OrgSlug turns an org name into a URL-safe slug.
func OrgSlug(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		case c == ' ', c == '-', c == '_':
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	if len(out) == 0 {
		return fmt.Sprintf("org-%d", time.Now().UnixNano())
	}
	return string(out)
}
