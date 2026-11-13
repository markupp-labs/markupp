package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/loginprovider"
)

// ExternalLoginService é o login por provedor externo consumido pelas rotas (ADR-0054).
type ExternalLoginService interface {
	Providers() []auth.LoginProviderSummary
	Begin(ctx context.Context, providerID string) (loginprovider.Attempt, error)
	Login(ctx context.Context, cb auth.ExternalCallback) (auth.TokenPair, error)
	Register(ctx context.Context, cb auth.ExternalCallback) (auth.TokenPair, error)
	Link(ctx context.Context, userID string, cb auth.ExternalCallback) (auth.UserIdentity, error)
	ListIdentities(ctx context.Context, userID string) ([]auth.UserIdentity, error)
	Unlink(ctx context.Context, userID, identityID string) error
}

type externalLoginHandler struct {
	svc ExternalLoginService
}

type loginProviderResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type loginAttemptResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	State            string `json:"state"`
	CodeVerifier     string `json:"code_verifier"`
	Nonce            string `json:"nonce"`
}

type externalCallbackRequest struct {
	Provider     string `json:"provider"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	Nonce        string `json:"nonce"`
}

type identityResponse struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`
	CreatedAt time.Time `json:"created_at"`
}

// mountExternalLoginRoutes monta o login externo, só sob /v1. As identidades da conta
// só aceitam token de login: um app não troca as formas de entrar de ninguém.
func mountExternalLoginRoutes(r chi.Router, svc ExternalLoginService, tokens TokenValidator, limit func(http.Handler) http.Handler) {
	h := &externalLoginHandler{svc: svc}
	r.Get("/login-providers", h.listProviders)
	r.With(limit).Post("/login-providers/{id}/attempts", h.createAttempt)
	r.With(limit).Post("/auth/external-login", h.login)
	r.With(limit).Post("/accounts/external", h.register)
	account := r.With(RequireAuth(tokens))
	account.Get("/me/identities", h.listIdentities)
	account.Post("/me/identities", h.link)
	account.Delete("/me/identities/{id}", h.unlink)
}

func (h *externalLoginHandler) listProviders(w http.ResponseWriter, _ *http.Request) {
	out := []loginProviderResponse{}
	for _, p := range h.svc.Providers() {
		out = append(out, loginProviderResponse{ID: p.ID, Name: p.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *externalLoginHandler) createAttempt(w http.ResponseWriter, r *http.Request) {
	attempt, err := h.svc.Begin(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, auth.ErrUnknownLoginProvider) {
		writeError(w, "not_found", "login provider not found", http.StatusNotFound)
		return
	}
	if err != nil {
		writeExternalLoginError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, loginAttemptResponse{
		AuthorizationURL: attempt.AuthorizationURL, State: attempt.State, CodeVerifier: attempt.CodeVerifier, Nonce: attempt.Nonce,
	})
}

func (h *externalLoginHandler) login(w http.ResponseWriter, r *http.Request) {
	h.openSession(w, r, h.svc.Login, http.StatusOK)
}

func (h *externalLoginHandler) register(w http.ResponseWriter, r *http.Request) {
	h.openSession(w, r, h.svc.Register, http.StatusCreated)
}

// openSession lê o callback, chama o serviço e devolve a sessão com status.
func (h *externalLoginHandler) openSession(w http.ResponseWriter, r *http.Request,
	call func(context.Context, auth.ExternalCallback) (auth.TokenPair, error), status int) {
	cb, ok := decodeCallback(w, r)
	if !ok {
		return
	}
	pair, err := call(r.Context(), cb)
	if err != nil {
		writeExternalLoginError(w, err)
		return
	}
	writeJSON(w, status, pair)
}

func decodeCallback(w http.ResponseWriter, r *http.Request) (auth.ExternalCallback, bool) {
	var req externalCallbackRequest
	if !decodeJSONBody(w, r, &req) {
		return auth.ExternalCallback{}, false
	}
	return auth.ExternalCallback{Provider: req.Provider, Code: req.Code, CodeVerifier: req.CodeVerifier, Nonce: req.Nonce}, true
}

func (h *externalLoginHandler) listIdentities(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	identities, err := h.svc.ListIdentities(r.Context(), actor.UserID)
	if err != nil {
		writeExternalLoginError(w, err)
		return
	}
	out := make([]identityResponse, 0, len(identities))
	for _, i := range identities {
		out = append(out, toIdentityResponse(i))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *externalLoginHandler) link(w http.ResponseWriter, r *http.Request) {
	cb, ok := decodeCallback(w, r)
	if !ok {
		return
	}
	actor, _ := identity.ActorFrom(r.Context())
	linked, err := h.svc.Link(r.Context(), actor.UserID, cb)
	if err != nil {
		writeExternalLoginError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toIdentityResponse(linked))
}

func (h *externalLoginHandler) unlink(w http.ResponseWriter, r *http.Request) {
	actor, _ := identity.ActorFrom(r.Context())
	if err := h.svc.Unlink(r.Context(), actor.UserID, chi.URLParam(r, "id")); err != nil {
		writeExternalLoginError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toIdentityResponse(i auth.UserIdentity) identityResponse {
	return identityResponse{ID: i.ID, Provider: i.Provider, CreatedAt: i.CreatedAt.UTC()}
}

// externalLoginErrors traduz os erros do login externo; a ordem importa só para quem
// embrulha mais de um.
var externalLoginErrors = []struct {
	err     error
	status  int
	code    string
	message string
}{
	{auth.ErrIdentityNotLinked, http.StatusUnauthorized, "identity_not_linked", "no account uses this identity; create an account with it or link it in the account settings"},
	{loginprovider.ErrLoginRejected, http.StatusUnauthorized, "login_rejected", "the login provider rejected this sign in; start again"},
	{loginprovider.ErrProviderUnavailable, http.StatusBadGateway, "provider_unavailable", "the login provider did not answer; try again later"},
	{auth.ErrUnknownLoginProvider, http.StatusBadRequest, "unknown_login_provider", "this installation has no such login provider"},
	{auth.ErrRegistrationDisabled, http.StatusForbidden, "registration_disabled", "this installation only creates accounts by invite"},
	{auth.ErrIdentityAlreadyLinked, http.StatusConflict, "identity_already_linked", "this identity already belongs to an account"},
	{auth.ErrUserAlreadyExists, http.StatusConflict, "already_exists", "this email already has an account; sign in and link the provider in the account settings"},
	{auth.ErrProviderEmailMissing, http.StatusUnprocessableEntity, "provider_email_missing", "the login provider did not share an email with this installation"},
	{auth.ErrIdentityNotFound, http.StatusNotFound, "not_found", "identity not found"},
	{auth.ErrLastIdentity, http.StatusConflict, "last_identity", "this is the last way to sign in to the account"},
}

func writeExternalLoginError(w http.ResponseWriter, err error) {
	for _, known := range externalLoginErrors {
		if errors.Is(err, known.err) {
			writeError(w, known.code, known.message, known.status)
			return
		}
	}
	writeAuthInternal(w, "login externo", err)
}
