package api

import (
	"net/http"
	"slices"
	"strings"
)

// exposedMethods são os métodos que a API atende, anunciados no preflight.
var exposedMethods = strings.Join([]string{
	http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions,
}, ", ")

const preflightMaxAge = "600"

// allowOrigins libera CORS só para as origens listadas, comparadas por igualdade
// exata. Lista vazia deixa o servidor sem nenhum cabeçalho de CORS.
func allowOrigins(origins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || !slices.Contains(origins, origin) {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			if !isPreflight(r) {
				next.ServeHTTP(w, r)
				return
			}
			writePreflight(w)
		})
	}
}

func isPreflight(r *http.Request) bool {
	return r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
}

func writePreflight(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", exposedMethods)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Max-Age", preflightMaxAge)
	w.WriteHeader(http.StatusNoContent)
}
