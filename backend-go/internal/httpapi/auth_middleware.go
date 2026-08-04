package httpapi

import (
	"net/http"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
)

// RequireSession allows Guest + User + Admin (any valid session)
func RequireSession(service *account.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := service.Resolve(r.Context(), sessionToken(r))
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": "Authentication required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin allows only Admin (password-authenticated administrators)
func RequireAdmin(service *account.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := service.Resolve(r.Context(), sessionToken(r))
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": "Authentication required"})
				return
			}
			if !session.Admin() {
				writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "Administrator access required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireNonGuest allows User + Admin (but not Guest)
func RequireNonGuest(service *account.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, err := service.Resolve(r.Context(), sessionToken(r))
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, map[string]string{"message": "Authentication required"})
				return
			}
			if session.Guest {
				writeErrorJSON(w, http.StatusForbidden, map[string]string{"message": "This feature requires a registered account"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
