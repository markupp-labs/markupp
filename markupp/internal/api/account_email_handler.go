package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
)

// AccountEmailService é a verificação de email e a recuperação de acesso da conta.
type AccountEmailService interface {
	RequestEmailVerification(ctx context.Context, userID string) error
	ConfirmEmail(ctx context.Context, userID, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, password string) error
}

type accountEmailHandler struct {
	svc AccountEmailService
}

type emailTokenRequest struct {
	Token string `json:"token"`
}

type passwordForgotRequest struct {
	Email string `json:"email"`
}

type passwordResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// mountAccountEmailRoutes expõe a verificação só à conta logada, que é a única a quem o
// token dela serve, e a troca de senha a quem tem o token do email.
func mountAccountEmailRoutes(r chi.Router, svc AccountEmailService, validator TokenValidator) {
	h := &accountEmailHandler{svc: svc}
	r.Post("/auth/password/forgot", h.forgotPassword)
	r.Post("/auth/password/reset", h.resetPassword)
	r.Group(func(g chi.Router) {
		g.Use(RequireAuth(validator))
		g.Post("/auth/email/verification", h.requestVerification)
		g.Post("/auth/email/verify", h.confirmEmail)
	})
}

func (h *accountEmailHandler) requestVerification(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	h.reply(w, h.svc.RequestEmailVerification(r.Context(), actor.UserID), http.StatusAccepted)
}

func (h *accountEmailHandler) confirmEmail(w http.ResponseWriter, r *http.Request) {
	var req emailTokenRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	h.reply(w, h.svc.ConfirmEmail(r.Context(), actor.UserID, req.Token), http.StatusNoContent)
}

// forgotPassword responde 202 exista a conta ou não.
func (h *accountEmailHandler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req passwordForgotRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	h.reply(w, h.svc.RequestPasswordReset(r.Context(), req.Email), http.StatusAccepted)
}

func (h *accountEmailHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	h.reply(w, h.svc.ResetPassword(r.Context(), req.Token, req.Password), http.StatusNoContent)
}

func (h *accountEmailHandler) reply(w http.ResponseWriter, err error, success int) {
	if err == nil {
		w.WriteHeader(success)
		return
	}
	writeAccountEmailError(w, err)
}

func writeAccountEmailError(w http.ResponseWriter, err error) {
	if writeCredentialInputError(w, err) {
		return
	}
	switch {
	case errors.Is(err, auth.ErrUserNotFound):
		writeError(w, "unauthorized", "token ausente ou invalido", http.StatusUnauthorized)
	case errors.Is(err, auth.ErrEmailAlreadyVerified):
		writeError(w, "email_already_verified", err.Error(), http.StatusConflict)
	case errors.Is(err, auth.ErrEmailUnavailable):
		writeError(w, "email_unavailable", err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, auth.ErrInvalidToken):
		writeError(w, "invalid_token", err.Error(), http.StatusBadRequest)
	default:
		writeInternal(w, "email da conta", err)
	}
}
