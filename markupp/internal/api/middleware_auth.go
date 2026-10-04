package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

type userCtxKey struct{}

// UserContext armazena a identidade do usuário autenticado no contexto HTTP.
type UserContext struct {
	UserID   string
	TenantID string
	Role     string
}

// TokenValidator define o contrato consumido pelo middleware para verificar o JWT.
type TokenValidator interface {
	ValidateToken(tokenString string) (auth.Claims, error)
}

// RequireAuth intercepta requisições exigindo um cabeçalho Authorization: Bearer <token>.
func RequireAuth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			ctx := context.WithValue(r.Context(), userCtxKey{}, UserContext{
				UserID:   claims.Subject,
				TenantID: claims.TenantID,
				Role:     claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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

// UserFromContext recupera os dados do usuário autenticado a partir do contexto.
func UserFromContext(ctx context.Context) (UserContext, bool) {
	u, ok := ctx.Value(userCtxKey{}).(UserContext)
	return u, ok
}
