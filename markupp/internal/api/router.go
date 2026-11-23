package api

import (
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/instance"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RouterDeps reúne o que o roteador expõe. As rotas de um serviço nil não são montadas.
type RouterDeps struct {
	Notes          NoteService
	Graph          NoteGraph
	Auth           AuthService
	Admin          AdminService
	Vaults         VaultService
	LinkRefusals   LinkRefusalService
	AccountEmail   AccountEmailService
	ExternalLogin  ExternalLoginService
	Tokens         TokenValidator
	Tenants        TenantResolver
	Probe          StorageProbe
	AllowedOrigins []string
	OAuthMetadata  *OAuthMetadata
	OAuth          OAuthService
	// MCPUpstreamURL é o servidor MCP para onde /mcp é encaminhado. Vazia, /mcp não existe.
	MCPUpstreamURL string
	// PanelUpstreamURL é o painel para onde vai todo caminho que o servidor não atende.
	// Vazia, a raiz não encaminha nada.
	PanelUpstreamURL string
	// TrustedProxies são as redes dos proxies cujo X-Forwarded-For os proxies do painel
	// e do MCP repassam. De outra conexão, o header de entrada é descartado.
	TrustedProxies []netip.Prefix
	// AccountRateLimiter limita as rotas de conta (ADR-0044). Nil deixa as rotas livres.
	AccountRateLimiter *AccountRateLimiter
	// Publications publica e despublica notas, pastas e cofres (ADR-0048).
	Publications PublicationService
	// PublicNotes lê as notas públicas sem conta, pelo id público.
	PublicNotes PublicNoteReader
	// PublicRateLimiter limita a leitura pública por IP. Nil deixa as rotas livres.
	PublicRateLimiter *AccountRateLimiter
	// PublicRelay confere o servidor MCP, cujo X-Forwarded-For conta no limite da leitura
	// pública como o IP do agente. Nil não confia em nenhum relay.
	PublicRelay RelayAuthenticator
	// InstanceAdmins são os emails das contas que administram a instância (ADR-0057).
	InstanceAdmins instance.Admins
	// InstanceEmbedding lê e grava a escolha do provedor de embedding. Sem ele, ou sem
	// Auth, as rotas da instância não existem.
	InstanceEmbedding InstanceEmbeddingService
}

// NewRouter monta o roteador com sonda, rotas de autenticação, de notas e de
// administração, servidas só sob /v1. As rotas do authorization server OAuth que os
// clientes acessam ficam na raiz, sem versão e fora da lista de origens do CORS, e as
// da tela de consentimento só sob /v1.
func NewRouter(d RouterDeps) chi.Router {
	mustHaveRouterDeps(d)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	mountOAuthServerRoutes(r, d)
	mountMCPProxy(r, d.MCPUpstreamURL, d.TrustedProxies)
	mountPanelProxy(r, d.PanelUpstreamURL, d.TrustedProxies)
	mountPublicNoteRoutes(r, d.PublicNotes, limitPublicReads(d.PublicRateLimiter, d.PublicRelay))
	r.Group(func(g chi.Router) {
		g.Use(allowOrigins(d.AllowedOrigins))
		mountAPIRoutes(g, d)
	})
	return r
}

// mountAPIRoutes monta as rotas sob /v1.
func mountAPIRoutes(r chi.Router, d RouterDeps) {
	if d.OAuth != nil {
		mountOAuthRequestRoutes(r, d.OAuth, d.Tokens)
		mountOAuthAuthorizationRoutes(r, d.OAuth, d.Tokens)
	}
	r.Mount(currentVersionPrefix, newVersionRoutes(d))
}

// mustHaveRouterDeps barra na montagem as dependências que faltam às rotas ligadas.
func mustHaveRouterDeps(d RouterDeps) {
	if d.Tokens == nil {
		panic("api: Tokens obrigatorio para proteger as rotas")
	}
	if d.Tenants == nil && (d.Notes != nil || d.Graph != nil || d.Admin != nil || d.Vaults != nil || d.Publications != nil || d.LinkRefusals != nil) {
		panic("api: Tenants obrigatorio para as rotas de notas, do grafo, de administracao, de cofres, de publicacao e de recusa de ligacao")
	}
	if d.OAuth != nil && d.OAuthMetadata == nil {
		panic("api: OAuthMetadata obrigatorio para as rotas OAuth, que usam o Issuer")
	}
}

// newVersionRoutes monta as rotas da versão atual.
func newVersionRoutes(d RouterDeps) chi.Router {
	r := chi.NewRouter()
	r.Get("/healthz", healthz(d.Probe))
	limit := limitAccountAttempts(d.AccountRateLimiter)
	if d.Auth != nil {
		mountAuthRoutes(r, &authHandler{svc: d.Auth, admins: d.InstanceAdmins}, d.Tokens, limit)
	}
	if d.Notes != nil {
		mountProtectedNotesRoutes(r, d.Notes, d.Tokens, d.Tenants)
	}
	mountV1OnlyRoutes(r, d)
	if d.Admin != nil {
		mountAdminRoutes(r, d.Admin, d.Tokens, d.Tenants)
	}
	if d.AccountEmail != nil {
		mountAccountEmailRoutes(r, d.AccountEmail, d.Tokens, limit)
	}
	return r
}

// mountV1OnlyRoutes monta as rotas que nasceram depois da transição do ADR-0026: o grafo, a gestão de cofres, a publicação, o login externo e a recusa
// de ligação, e as da instância.
func mountV1OnlyRoutes(r chi.Router, d RouterDeps) {
	if d.Graph != nil {
		mountProtectedGraphRoutes(r, d.Graph, d.Tokens, d.Tenants)
	}
	if d.Vaults != nil {
		mountVaultRoutes(r, d.Vaults, d.Tokens, d.Tenants)
	}
	if d.ExternalLogin != nil {
		mountExternalLoginRoutes(r, d.ExternalLogin, d.Tokens, limitAccountAttempts(d.AccountRateLimiter))
	}
	if d.Publications != nil {
		mountPublicationRoutes(r, d.Publications, d.Tokens, d.Tenants)
	}
	if d.LinkRefusals != nil {
		mountLinkRefusalRoutes(r, d.LinkRefusals, d.Tokens, d.Tenants)
	}
	if d.InstanceEmbedding != nil && d.Auth != nil {
		mountInstanceRoutes(r, d.InstanceEmbedding, d.Auth, d.InstanceAdmins, d.Tokens)
	}
}

// mountProtectedNotesRoutes confere o escopo do token antes de resolver o tenant, para
// falhar cedo, como em /users.
func mountProtectedNotesRoutes(r chi.Router, svc NoteService, tokenValidator TokenValidator, tenants TenantResolver) {
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(tokenValidator), RequireTenant(tenants))
	write := r.With(acceptOAuthScope(oauth.ScopeNotesWrite), RequireAuth(tokenValidator), RequireTenant(tenants), requireWriter)
	legacy := &notesHandler{svc: svc, render: toResponse}
	read.Get("/notes", legacy.list)
	read.Get("/notes/search", legacy.search)
	write.Post("/notes", legacy.create)
	mountV1NotesRoutes(read, write, svc)
}

// mountNoteItemRoutes monta a leitura, a edição e a remoção da nota de id global.
func mountNoteItemRoutes(read, write chi.Router, h *notesHandler) {
	read.Get("/notes/{id}", h.get)
	write.Put("/notes/{id}", h.update)
	write.Delete("/notes/{id}", h.delete)
}

// mountProtectedGraphRoutes monta o grafo, que nasceu depois da transição e existe só
// sob /v1, lido com o escopo de leitura das notas (emenda do ADR-0040).
func mountProtectedGraphRoutes(r chi.Router, graph NoteGraph, tokenValidator TokenValidator, tenants TenantResolver) {
	mountNoteGraphRoutes(r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(tokenValidator), RequireTenant(tenants)), graph)
}

// mountAuthRoutes monta login, sessão e as rotas da conta. As notificações trazem os convites de
// GET /me/invites, então pedem o mesmo escopo administracao ao token OAuth.
func mountAuthRoutes(r chi.Router, h *authHandler, tokenValidator TokenValidator, limit func(http.Handler) http.Handler) {
	r.With(limit).Post("/auth/login", h.login)
	r.With(limit).Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.With(limit).Post("/auth/invites/accept", h.acceptInvite)
	account := r.With(acceptOAuthScope(anyOAuthScope), RequireAuth(tokenValidator))
	account.Get("/auth/me", h.me)
	account.Get("/me/tenants", h.myTenants)
	admin := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(tokenValidator))
	admin.Post("/tenants", h.createTenant)
	admin.Get("/me/invites", h.myInvites)
	admin.Post("/me/invites/{id}/accept", h.acceptMyInvite)
	admin.Post("/me/invites/{id}/decline", h.declineMyInvite)
	admin.Get("/me/notifications", h.myNotifications)
	r.With(limit).Post("/accounts", h.createAccount)
}
