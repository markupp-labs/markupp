package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
)

// RouterDeps reúne o que o roteador expõe. As rotas de um serviço nil não são montadas.
type RouterDeps struct {
	Notes          NoteService
	Graph          NoteGraph
	Auth           AuthService
	Admin          AdminService
	AccountEmail   AccountEmailService
	Tokens         TokenValidator
	Tenants        TenantResolver
	Probe          StorageProbe
	AllowedOrigins []string
	OAuthMetadata  *OAuthMetadata
	OAuth          OAuthService
	// MCPUpstreamURL é o servidor MCP para onde /mcp é encaminhado. Vazia, /mcp não existe.
	MCPUpstreamURL string
}

// NewRouter monta o roteador com sonda, rotas de autenticação, de notas e de
// administração, servidas sob /v1 e, durante a transição do ADR-0026, também sem
// prefixo e marcadas como obsoletas. As rotas do authorization server OAuth que os
// clientes acessam ficam na raiz, sem versão e fora da lista de origens do CORS, e as
// da tela de consentimento só sob /v1.
func NewRouter(d RouterDeps) chi.Router {
	mustHaveRouterDeps(d)
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	mountOAuthServerRoutes(r, d)
	mountMCPProxy(r, d.MCPUpstreamURL)
	r.Group(func(g chi.Router) {
		g.Use(allowOrigins(d.AllowedOrigins))
		mountAPIRoutes(g, d)
	})
	return r
}

// mountAPIRoutes monta as rotas sob /v1 e o alias obsoleto sem prefixo, que responde
// por todo caminho que nenhuma outra rota atende.
func mountAPIRoutes(r chi.Router, d RouterDeps) {
	if d.OAuth != nil {
		mountOAuthRequestRoutes(r, d.OAuth, d.Tokens)
		mountOAuthAuthorizationRoutes(r, d.OAuth, d.Tokens)
	}
	r.Mount(currentVersionPrefix, newVersionRoutes(d, true))
	r.Mount("/", markDeprecated(newVersionRoutes(d, false)))
}

// mustHaveRouterDeps barra na montagem as dependências que faltam às rotas ligadas.
func mustHaveRouterDeps(d RouterDeps) {
	if d.Tokens == nil {
		panic("api: Tokens obrigatorio para proteger as rotas")
	}
	if d.Tenants == nil && (d.Notes != nil || d.Graph != nil || d.Admin != nil) {
		panic("api: Tenants obrigatorio para as rotas de notas, do grafo e de administracao")
	}
	if d.OAuth != nil && d.OAuthMetadata == nil {
		panic("api: OAuthMetadata obrigatorio para as rotas OAuth, que usam o Issuer")
	}
}

// newVersionRoutes monta as rotas da versão atual (current) ou do alias obsoleto sem
// prefixo, que não ganha as rotas nascidas depois da transição.
func newVersionRoutes(d RouterDeps, current bool) chi.Router {
	r := chi.NewRouter()
	r.Get("/healthz", healthz(d.Probe))
	if d.Auth != nil {
		mountAuthRoutes(r, d.Auth, d.Tokens)
	}
	if d.Notes != nil {
		mountProtectedNotesRoutes(r, d.Notes, d.Tokens, d.Tenants, current)
	}
	if d.Graph != nil && current {
		mountProtectedGraphRoutes(r, d.Graph, d.Tokens, d.Tenants)
	}
	if d.Admin != nil {
		mountAdminRoutes(r, d.Admin, d.Tokens, d.Tenants)
	}
	if d.AccountEmail != nil {
		mountAccountEmailRoutes(r, d.AccountEmail, d.Tokens)
	}
	return r
}

// mountProtectedNotesRoutes confere o escopo do token antes de resolver o tenant, para
// falhar cedo, como em /users.
// current liga a representação de /v1 (ADR-0040) no item da nota, nas notas do cofre e
// na busca; o alias sem prefixo mantém a representação antiga.
func mountProtectedNotesRoutes(r chi.Router, svc NoteService, tokenValidator TokenValidator, tenants TenantResolver, current bool) {
	read := r.With(acceptOAuthScope(oauth.ScopeNotesRead), RequireAuth(tokenValidator), RequireTenant(tenants))
	write := r.With(acceptOAuthScope(oauth.ScopeNotesWrite), RequireAuth(tokenValidator), RequireTenant(tenants), requireWriter)
	legacy := &notesHandler{svc: svc, render: toResponse}
	read.Get("/notes", legacy.list)
	read.Get("/notes/search", legacy.search)
	write.Post("/notes", legacy.create)
	if !current {
		mountNoteItemRoutes(read, write, legacy)
		return
	}
	mountNoteItemRoutes(read, write, &notesHandler{svc: svc, render: toV1Response})
	mountV1NotesRoutes(r.With(RequireAuth(tokenValidator), RequireTenant(tenants)), svc)
}

// mountNoteItemRoutes monta a leitura, a edição e a remoção da nota de id global.
func mountNoteItemRoutes(read, write chi.Router, h *notesHandler) {
	read.Get("/notes/{id}", h.get)
	write.Put("/notes/{id}", h.update)
	write.Delete("/notes/{id}", h.delete)
}

// mountProtectedGraphRoutes monta o grafo, que nasceu depois da transição e existe só
// sob /v1; sem escopo OAuth declarado, aceita só o token de login.
func mountProtectedGraphRoutes(r chi.Router, graph NoteGraph, tokenValidator TokenValidator, tenants TenantResolver) {
	mountNoteGraphRoutes(r.With(RequireAuth(tokenValidator), RequireTenant(tenants)), graph)
}

func mountAuthRoutes(r chi.Router, authSvc AuthService, tokenValidator TokenValidator) {
	h := &authHandler{svc: authSvc}
	r.Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.Post("/auth/invites/accept", h.acceptInvite)
	account := r.With(acceptOAuthScope(anyOAuthScope), RequireAuth(tokenValidator))
	account.Get("/auth/me", h.me)
	account.Get("/me/tenants", h.myTenants)
	admin := r.With(acceptOAuthScope(oauth.ScopeAdmin), RequireAuth(tokenValidator))
	admin.Post("/tenants", h.createTenant)
	admin.Get("/me/invites", h.myInvites)
	admin.Post("/me/invites/{id}/accept", h.acceptMyInvite)
	admin.Post("/me/invites/{id}/decline", h.declineMyInvite)
}
