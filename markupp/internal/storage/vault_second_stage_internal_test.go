package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// revisaoNaFila lê a revisão da nota noteID na fila do indexador.
func revisaoNaFila(t *testing.T, pool *pgxpool.Pool, noteID string) int64 {
	t.Helper()
	var revisao int64
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT revision FROM note_index_queue WHERE note_id = $1", noteID).Scan(&revisao))
	return revisao
}

// refazDfNoCofreInterno roda a troca do df de cofre-interno no papel do indexador.
func refazDfNoCofreInterno(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	cofre := gen.ReservePendingVaultRow{VaultID: "cofre-interno", TenantID: "default"}
	require.NoError(t, inTx(context.Background(), pool, func(q *gen.Queries) error {
		if err := enterVault(context.Background(), q, cofre); err != nil {
			return err
		}
		return refreshTerms(context.Background(), q, cofre.VaultID)
	}))
}

func TestSegundoEstagio_CofreSemDocumento_NaoReenfileiraAsNotas(t *testing.T) {
	pool := bancoInternoMigrado(t)
	agora := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(context.Background(), inserirNotaInterna, "pendente", "pendente.md", "texto", agora, agora)
	require.NoError(t, err)
	antes := revisaoNaFila(t, pool, "pendente")

	refazDfNoCofreInterno(t, pool)
	refazDfNoCofreInterno(t, pool)

	assert.Equal(t, antes, revisaoNaFila(t, pool, "pendente"),
		"df com N zero nao tem termo para reescolher; a nota ainda sem documento ja esta na fila e nao ganha revisao nova")
}
