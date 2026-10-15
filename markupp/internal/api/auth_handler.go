package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
)

const maxAuthBodyBytes = 4096

// AuthService é a interface do serviço de autenticação consumida pelos endpoints HTTP.
type AuthService interface {
	LoginLocal(ctx context.Context, email, password string) (auth.TokenPair, error)
	RefreshToken(ctx context.Context, rawRefreshToken string) (auth.TokenPair, error)
	RevokeSession(ctx context.Context, rawRefreshToken string) error
	GetAccount(ctx context.Context, userID string) (auth.User, error)
}

type authHandler struct {
	svc    AuthService
	logger *slog.Logger
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	pair, err := h.svc.LoginLocal(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	pair, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pair)
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid_request", "JSON invalido", http.StatusBadRequest)
		return
	}
	if err := h.svc.RevokeSession(r.Context(), req.RefreshToken); err != nil {
		h.logError("falha ao revogar sessao", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) logError(msg string, err error) {
	logger := h.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(msg, "error", err)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	u, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "nao autenticado", http.StatusUnauthorized)
		return
	}
	user, err := h.svc.GetAccount(r.Context(), u.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			writeError(w, "not_found", "usuario nao encontrado", http.StatusNotFound)
			return
		}
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, userResponse{
		ID:       user.ID,
		TenantID: user.TenantID,
		Email:    user.Email,
		Role:     user.Role,
	})
}

func (h *authHandler) handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrEmptyEmail), errors.Is(err, auth.ErrEmptyPassword), errors.Is(err, auth.ErrPasswordTooLong):
		writeError(w, "invalid_request", err.Error(), http.StatusBadRequest)
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, "invalid_credentials", "credenciais invalidas", http.StatusUnauthorized)
	case errors.Is(err, auth.ErrLocalAuthDisabled):
		writeError(w, "local_auth_disabled", "autenticacao local desativada", http.StatusForbidden)
	case errors.Is(err, auth.ErrRefreshTokenRevoked), errors.Is(err, auth.ErrRefreshTokenExpired), errors.Is(err, auth.ErrRefreshTokenInvalid):
		writeError(w, "invalid_refresh_token", "refresh token revogado ou expirado", http.StatusUnauthorized)
	default:
		writeError(w, "internal", "erro interno", http.StatusInternalServerError)
	}
}
