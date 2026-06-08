package api

import (
	"net/http"
	"time"
)

const sessionCookieName = "gm_session"

func setSessionCookie(w http.ResponseWriter, token, expiresAtRFC3339 string, secure bool) {
	exp, err := time.Parse(time.RFC3339, expiresAtRFC3339)
	if err != nil {
		exp = time.Now().Add(30 * 24 * time.Hour)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func sessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}
