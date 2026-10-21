package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
)

// Caminhos do servidor MCP na mesma origem (ADR-0041): o endpoint Streamable HTTP e a
// metadata do recurso protegido dele (RFC 9728), que o próprio servidor MCP publica.
const (
	mcpEndpointPath         = "/mcp"
	mcpResourceMetadataPath = "/.well-known/oauth-protected-resource/mcp"
)

// mountMCPProxy encaminha o servidor MCP ao container dele, sem CORS nem autenticação: o
// servidor MCP troca o token do agente e responde 401 sozinho.
func mountMCPProxy(r chi.Router, upstreamURL string) {
	if upstreamURL == "" {
		return
	}
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		panic("api: MCPUpstreamURL invalida, a config deveria ter recusado: " + err.Error())
	}
	proxy := withoutWriteDeadline(newMCPReverseProxy(upstream))
	r.Handle(mcpEndpointPath, proxy)
	r.Handle(mcpResourceMetadataPath, proxy)
}

// newMCPReverseProxy repassa cada pedaço da resposta assim que chega, porque o
// Streamable HTTP responde por SSE.
func newMCPReverseProxy(upstream *url.URL) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.SetXForwarded()
		},
		FlushInterval: -1,
	}
}

// withoutWriteDeadline tira desta resposta o WriteTimeout do servidor, que cortaria o
// stream SSE aberto pelo agente; o resto da API segue com o prazo.
func withoutWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		next.ServeHTTP(w, r)
	})
}
