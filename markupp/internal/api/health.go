package api

import (
	"context"
	"net/http"
)

// StorageProbe reporta se o armazenamento que sustenta a API responde.
type StorageProbe interface {
	PingContext(ctx context.Context) error
}

type healthResponse struct {
	Status string `json:"status"`
}

// healthz atende a sonda do orquestrador consultando o armazenamento.
func healthz(probe StorageProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := probe.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}
