package web

import (
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"

	"github.com/jhash/tabitha/internal/auth"
	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
)

// configureGoogleAuth registers goth's Google provider and gothic's session
// store. It's a no-op when Google credentials aren't configured, so a dev
// environment without them still boots — just without login.
//
// Scopes are explicit: passing any scopes to google.New disables goth's
// default "email" scope, so "email" and "profile" must be listed alongside
// the read-only Drive/Docs scope tabitha needs for Jeff's documents.
func configureGoogleAuth(cfg config.Config) {
	if !auth.GoogleConfigured(cfg) {
		return
	}

	gothic.Store = sessions.NewCookieStore([]byte(cfg.SessionSecret))

	callbackURL := cfg.AppURL + "/auth/google/callback"
	provider := google.New(cfg.GoogleKey, cfg.GoogleSecret, callbackURL, "email", "profile", auth.GoogleDriveReadonlyScope)
	// Google only returns a refresh_token when access_type=offline and
	// prompt=consent are both explicit on the auth URL — otherwise a
	// returning user's re-login silently omits it, and the stored token
	// becomes unrefreshable once the access token expires (~1h).
	provider.SetAccessType("offline")
	provider.SetPrompt("consent")
	goth.UseProviders(provider)
}

// mountAuthRoutes wires the login/callback/logout routes. These are
// provider-agnostic (goth's own {provider} URL param), so adding a second
// provider later needs no route changes — only another entry in
// configureGoogleAuth's goth.UseProviders call.
func mountAuthRoutes(r chi.Router, cfg config.Config, q *db.Queries) {
	secureCookies := strings.HasPrefix(cfg.AppURL, "https://")

	r.Get("/auth/{provider}", gothic.BeginAuthHandler)
	r.Get("/auth/{provider}/callback", authCallbackHandler(cfg, q, secureCookies))
	r.Get("/auth/logout", authLogoutHandler(q, secureCookies))
}

func authCallbackHandler(cfg config.Config, q *db.Queries, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gothUser, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Printf("tabitha: oauth callback failed: %v", err)
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}

		key, err := auth.ParseEncryptionKey(cfg.TokenEncryptionKey)
		if err != nil {
			log.Printf("tabitha: invalid TOKEN_ENCRYPTION_KEY: %v", err)
			http.Error(w, "server misconfigured", http.StatusInternalServerError)
			return
		}

		_, token, expiresAt, err := auth.CompleteLogin(r.Context(), q, key, gothUser)
		if err != nil {
			log.Printf("tabitha: completing login: %v", err)
			http.Error(w, "authentication failed", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    token,
			Expires:  expiresAt,
			Path:     "/",
			HttpOnly: true,
			Secure:   secureCookies,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/admin", http.StatusFound)
	}
}

func authLogoutHandler(q *db.Queries, secureCookies bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
			_ = q.DeleteSession(r.Context(), cookie.Value)
		}
		_ = gothic.Logout(w, r)

		http.SetCookie(w, &http.Cookie{
			Name:     auth.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   secureCookies,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
