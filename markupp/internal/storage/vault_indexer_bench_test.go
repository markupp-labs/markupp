package storage_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/benchfixture"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// loteIncremental é o número de notas editadas entre duas execuções do Job.
const loteIncremental = 100

// reenfileiraNotas põe na fila count notas do cofre a partir de offset, como o
// trigger faria numa edição, sem mudar o conteúdo: o documento indexado não cresce
// de uma iteração para outra.
const reenfileiraNotas = `INSERT INTO note_index_queue (note_id)
SELECT id FROM notes ORDER BY id OFFSET $1 LIMIT $2
ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq')`

func BenchmarkIndexNextVault_CofreInteiro(b *testing.B) {
	benchIndexacao(b, benchfixture.VaultSize)
}

func BenchmarkIndexNextVault_LoteIncremental(b *testing.B) {
	benchIndexacao(b, loteIncremental)
}

// marcaCofre marca o cofre para o segundo estágio, como a exclusão de nota faria.
const marcaCofre = `INSERT INTO vault_second_stage_queue (vault_id) VALUES ('cofre-bench')
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq')`

// citacoesComErro é quantas notas do cofre sintético ganham uma ligação não resolvida
// para o segundo estágio inferir.
const citacoesComErro = 500

// citaComErroDeDigitacao faz a nota de posição i citar o título da nota seguinte com
// a primeira letra trocada, para cada uma das $1 primeiras notas, como o indexador
// gravaria a ligação escrita com erro.
const citaComErroDeDigitacao = `INSERT INTO note_links (note_id, vault_id, cited_title, title_key, kind)
SELECT n.id, n.vault_id, 'x' || substr(t.title_key, 2), 'x' || substr(t.title_key, 2), 'explicit'
FROM (SELECT id, vault_id, row_number() OVER (ORDER BY id) AS pos FROM notes) n
JOIN (SELECT title_key, row_number() OVER (ORDER BY note_id, title_key) AS pos FROM note_titles) t ON t.pos = n.pos + 1
WHERE n.pos <= $1`

// BenchmarkSegundoEstagio_CofreInteiro mede o segundo estágio sobre o cofre de 10 mil
// notas já indexado, com 500 ligações não resolvidas, num Job sem nota pendente: df,
// ligações inferidas, PageRank, graus e comunidades. O ANALYZE antes da medida é o
// caso comum; o primeiro Job de um cofre recém-importado roda sem estatística da
// carga, e as consultas por título ficam mais lentas.
func BenchmarkSegundoEstagio_CofreInteiro(b *testing.B) {
	benchSegundoEstagio(b, benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize))
}

// BenchmarkSegundoEstagio_CofreUniforme mede o mesmo no cofre uniforme, o pior caso:
// títulos de mesmo formato e ligações espalhadas sem hub.
func BenchmarkSegundoEstagio_CofreUniforme(b *testing.B) {
	benchSegundoEstagio(b, benchfixture.GenerateUniformVault(benchfixture.VaultSeed, benchfixture.VaultSize))
}

func benchSegundoEstagio(b *testing.B, vault []benchfixture.Note) {
	pool, idx := cofreIndexado(b, vault)
	_, err := pool.Exec(context.Background(), citaComErroDeDigitacao, citacoesComErro)
	require.NoError(b, err)
	_, err = pool.Exec(context.Background(), "ANALYZE")
	require.NoError(b, err, "estatisticas da carga, como o autovacuum deixaria entre dois Jobs")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		_, err := pool.Exec(context.Background(), marcaCofre)
		require.NoError(b, err)
		b.StartTimer()
		indexaLote(b, idx)
	}
}

// benchIndexacao mede IndexNextVault sobre lotes de porLote notas do cofre de 10
// mil notas já indexado, e reporta o custo por nota do lote.
func benchIndexacao(b *testing.B, porLote int) {
	pool, idx := cofreIndexado(b, benchfixture.GenerateVault(benchfixture.VaultSeed, benchfixture.VaultSize))
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		b.StopTimer()
		enfileiraLote(b, pool, i*porLote%benchfixture.VaultSize, porLote)
		b.StartTimer()
		indexaLote(b, idx)
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*porLote), "ns/nota")
}

// cofreIndexado semeia o cofre sintético vault inteiro e esvazia a fila dele: o
// primeiro segundo estágio reenfileira todas as notas para reescolher os termos com o
// idf, e sem esvaziar de novo a primeira medida pagaria um lote do cofre inteiro.
func cofreIndexado(b *testing.B, vault []benchfixture.Note) (*pgxpool.Pool, *storage.PostgresVaultIndexer) {
	b.Helper()
	pool := storagetest.EmptyPool(b)
	require.NoError(b, storage.Migrate(context.Background(), pool))
	ator := identity.Actor{UserID: "u-bench", TenantID: "t-bench", Role: "member"}
	storagetest.SeedVault(b, pool, ator, "cofre-bench", vault)
	idx, err := storage.NewPostgresVaultIndexer(context.Background(), pool, "simple", slog.New(slog.DiscardHandler))
	require.NoError(b, err)
	for found := true; found; {
		found, err = idx.IndexNextVault(context.Background(), indexing.Index)
		require.NoError(b, err)
	}
	return pool, idx
}

func enfileiraLote(b *testing.B, pool *pgxpool.Pool, offset, count int) {
	b.Helper()
	_, err := pool.Exec(context.Background(), reenfileiraNotas, offset, count)
	require.NoError(b, err, "reenfileirar %d notas a partir da %d", count, offset)
}

func indexaLote(b *testing.B, idx *storage.PostgresVaultIndexer) {
	b.Helper()
	found, err := idx.IndexNextVault(context.Background(), indexing.Index)
	require.NoError(b, err)
	require.True(b, found, "o lote enfileirado deveria deixar o cofre pendente")
}
