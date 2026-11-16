package api

import (
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// mountPanelProxy encaminha ao painel todo caminho que o servidor não atende, para o
// painel viver na raiz da mesma origem. O Host original segue para o painel gerar as
// URLs e conferir a origem dos formulários com o endereço que o navegador vê.
func mountPanelProxy(r chi.Router, upstreamURL string) {
	if upstreamURL == "" {
		return
	}
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		panic("api: invalid PanelUpstreamURL, the config should have rejected it: " + err.Error())
	}
	r.Handle("/*", &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(upstream)
		pr.Out.Host = pr.In.Host
		pr.SetXForwarded()
	}})
}
