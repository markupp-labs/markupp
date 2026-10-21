package storage_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

// poolDeUmaConexao abre sobre o banco de db um pool de uma conexão só, para a
// consulta preparada pela leitura ficar visível em pg_prepared_statements.
func poolDeUmaConexao(t *testing.T, db *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	cfg := db.Config()
	cfg.MaxConns, cfg.MinConns = 1, 0
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// planoGenerico devolve as linhas do plano genérico, o que o servidor reusa entre
// execuções, da consulta cujo nome casa com o padrão LIKE consulta e que a leitura
// já preparou na única conexão de pool. Varredura sequencial e ordenação inteira ficam desligadas: o plano só as usa
// quando a consulta não tem outro caminho.
func planoGenerico(t *testing.T, pool *pgxpool.Pool, consulta string) []string {
	t.Helper()
	ctx := context.Background()
	var texto string
	require.NoError(t, pool.QueryRow(ctx, "SELECT statement FROM pg_prepared_statements WHERE statement LIKE $1",
		"-- name: "+consulta+" %").Scan(&texto), "a leitura deveria ter preparado %s", consulta)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SET LOCAL enable_seqscan = off; SET LOCAL enable_sort = off")
	require.NoError(t, err)
	// O texto traz os parâmetros $n sem valor: vai pelo protocolo simples, cru.
	resultados, err := tx.Conn().PgConn().Exec(ctx, "EXPLAIN (GENERIC_PLAN, COSTS off) "+texto).ReadAll()
	require.NoError(t, err)
	var linhas []string
	for _, linha := range resultados[0].Rows {
		linhas = append(linhas, strings.TrimSpace(string(linha[0])))
	}
	return linhas
}

func TestNoteGraph_VaultLinks_PlanoLeSoAPaginaNaOrdemDoCaminho(t *testing.T) {
	db, _, _ := cofreLigado(t)
	pool := poolDeUmaConexao(t, db)
	_, err := storage.NewPostgresNoteGraph(pool, "simple").VaultLinks(ctxDeTeste, cofreDeTeste, notes.VaultLinkQuery{LinkFilter: todas})
	require.NoError(t, err)

	plano := planoGenerico(t, pool, "ListVaultLinks")

	require.GreaterOrEqual(t, len(plano), 2, "plano: %v", plano)
	assert.Equal(t, "->  Incremental Sort", plano[1],
		"as notas saem do indice na ordem do caminho e a leitura para na pagina, sem ordenar todas as ligacoes: %s", strings.Join(plano, "\n"))
}

func TestListVaultNotes_Orfas_PlanoGenericoUsaOIndiceParcial(t *testing.T) {
	db, _, _ := cofreLigado(t)
	pool := poolDeUmaConexao(t, db)
	_, err := storage.NewPostgresNotesRepository(pool, "simple").ListVaultNotes(ctxDeTeste, cofreDeTeste,
		notes.VaultNotesQuery{Orphan: ptr(true), Limit: 10})
	require.NoError(t, err)

	plano := strings.Join(planoGenerico(t, pool, "ListVault%Notes"), "\n")

	assert.Contains(t, plano, "note_centrality_vault_orphan_idx",
		"o plano reusado entre execucoes tambem le so as notas de grau zero do cofre")
}
