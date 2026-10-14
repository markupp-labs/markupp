package api

import (
	"net/http"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// TokenValidator define o contrato consumido pelo middleware para verificar o JWT.
type TokenValidator interface {
	ValidateToken(tokenString string) (auth.Claims, error)
}

// RequireAuth intercepta requisições exigindo um cabeçalho Authorization: Bearer <token>.
func RequireAuth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveAuthenticated(w, r, next, validator)
		})
	}
}

func serveAuthenticated(w http.ResponseWriter, r *http.Request, next http.Handler, validator TokenValidator) {
	tokenString, ok := extractBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, "unauthorized", "token ausente ou invalido", http.StatusUnauthorized)
		return
	}
	claims, err := validator.ValidateToken(tokenString)
	if err != nil {
		writeError(w, "unauthorized", "token ausente ou invalido", http.StatusUnauthorized)
		return
	}
	ctx := identity.WithActor(r.Context(), identity.Actor{
		UserID:   claims.Subject,
		TenantID: claims.TenantID,
		Role:     claims.Role,
	})
	next.ServeHTTP(w, r.WithContext(ctx))
}

func extractBearerToken(authHeader string) (string, bool) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}
