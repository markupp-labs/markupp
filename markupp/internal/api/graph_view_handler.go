package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
)

// graphCompressionLevel é o nível do gzip das rotas do grafo, sem paginação.
const graphCompressionLevel = 5

// localGraphQuery é o pedido do grafo local: a profundidade e o filtro.
type localGraphQuery struct {
	depth  int
	filter notes.GraphFilter
}

// mountGraphViewRoutes monta o grafo para desenhar, do cofre e local, comprimido
// quando o cliente aceita e validado pela versão do grafo do cofre.
func mountGraphViewRoutes(r chi.Router, graph NoteGraph) {
	g := r.With(middleware.Compress(graphCompressionLevel))
	g.Get("/v1/vaults/{id}/graph", serveGraph(readGraphFilter, graph.VaultGraphVersion, graph.VaultGraph))
	g.Get("/v1/notes/{id}/graph", serveGraph(readLocalGraphQuery, graph.NoteGraphVersion,
		func(ctx context.Context, noteID string, q localGraphQuery) (notes.Graph, error) {
			return graph.LocalGraph(ctx, noteID, q.depth, q.filter)
		}))
}

// readGraphFilter lê os filtros: false esconde, true ou ausente não.
func readGraphFilter(qr *queryReader) notes.GraphFilter {
	return notes.GraphFilter{
		HideOrphans:    isFalse(qr.boolean("orphans")),
		HideUnresolved: isFalse(qr.boolean("unresolved")),
		HideInferred:   isFalse(qr.boolean("inferred")),
		Community:      qr.int64("community"),
	}
}

func isFalse(b *bool) bool {
	return b != nil && !*b
}

func readLocalGraphQuery(qr *queryReader) localGraphQuery {
	depth := qr.intBetween("depth", notes.DefaultGraphDepth, notes.MinGraphDepth, notes.MaxGraphDepth)
	return localGraphQuery{depth: depth, filter: readGraphFilter(qr)}
}

// serveGraph responde com o grafo que load lê para o id da rota e a consulta que read
// lê da query string. A versão vem antes do grafo e vira o ETag: o grafo lido depois
// é no mínimo tão novo quanto ela, e com If-None-Match igual o grafo nem é lido. O
// grafo é do usuário e muda a cada segundo estágio: só o navegador dele guarda, e
// revalida pelo ETag antes de usar.
func serveGraph[Q any](read func(*queryReader) Q, version func(ctx context.Context, id string) (int64, error), load func(ctx context.Context, id string, q Q) (notes.Graph, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, ok := readQuery(w, r, read)
		if !ok {
			return
		}
		id := chi.URLParam(r, "id")
		v, err := version(r.Context(), id)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		w.Header().Set("Cache-Control", "private, no-cache")
		if notModified(w, r, v) {
			return
		}
		graph, err := load(r.Context(), id, q)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, graph)
	}
}

// readQuery lê a consulta com read e responde 400 quando ela é inválida.
func readQuery[Q any](w http.ResponseWriter, r *http.Request, read func(*queryReader) Q) (Q, bool) {
	qr := queryReader{values: r.URL.Query()}
	q := read(&qr)
	if qr.err != nil {
		writeDomainError(w, qr.err)
		return q, false
	}
	return q, true
}

// notModified põe o ETag fraco da versão do grafo e responde 304 quando o cliente já
// tem essa versão. A versão zero, antes do primeiro segundo estágio, não valida nada.
func notModified(w http.ResponseWriter, r *http.Request, version int64) bool {
	if version == 0 {
		return false
	}
	tag := fmt.Sprintf(`"%d"`, version)
	w.Header().Set("ETag", "W/"+tag)
	if !matchesETag(r.Header.Get("If-None-Match"), tag) {
		return false
	}
	w.WriteHeader(http.StatusNotModified)
	return true
}

// matchesETag compara If-None-Match com tag pela comparação fraca, que ignora W/.
func matchesETag(ifNoneMatch, tag string) bool {
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == tag {
			return true
		}
	}
	return false
}
