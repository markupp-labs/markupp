package indexing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
)

func TestIndex_NotasMudadas_DevolveUmDocumentoPorNotaComOConteudo(t *testing.T) {
	cs := indexing.Changeset{TenantID: "t1", VaultID: "va", Notes: []indexing.NoteChange{
		{NoteID: "n1", Content: "reuniao com o cliente"},
		{NoteID: "n2", Content: "decisao tecnica"},
	}}

	docs := indexing.Index(cs)

	assert.Equal(t, []indexing.SearchDocument{
		{NoteID: "n1", Text: "reuniao com o cliente"},
		{NoteID: "n2", Text: "decisao tecnica"},
	}, docs)
}

func TestIndex_SemNotas_DevolveVazio(t *testing.T) {
	assert.Empty(t, indexing.Index(indexing.Changeset{TenantID: "t1", VaultID: "va"}))
}

// fakeVaultQueue faz o papel do banco: devolve, chamada a chamada, se havia cofre
// pendente, e aplica a função de indexação recebida a um lote fixo para o teste
// conferir qual função chegou.
type fakeVaultQueue struct {
	pending  []bool
	err      error
	calls    int
	produced []indexing.SearchDocument
}

func (f *fakeVaultQueue) IndexNextVault(_ context.Context, index indexing.IndexFunc) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if len(f.pending) == 0 {
		return false, nil
	}
	found := f.pending[0]
	f.pending = f.pending[1:]
	if found {
		f.produced = index(indexing.Changeset{Notes: []indexing.NoteChange{{NoteID: "n1", Content: "oi"}}})
	}
	return found, nil
}

// fakeWait conta as esperas entre rodadas e cancela depois de stopAfter delas.
type fakeWait struct {
	calls     int
	stopAfter int
}

func (f *fakeWait) wait(context.Context) error {
	f.calls++
	if f.calls >= f.stopAfter {
		return context.Canceled
	}
	return nil
}

func TestRunLoop_EsvaziaOsCofresPendentesAntesDeEsperar(t *testing.T) {
	queue := &fakeVaultQueue{pending: []bool{true, true, false, false}}
	espera := &fakeWait{stopAfter: 2}

	err := indexing.RunLoop(context.Background(), queue, espera.wait)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, queue.calls, "dois cofres, uma rodada vazia, espera, outra rodada vazia")
	assert.Equal(t, 2, espera.calls)
	assert.Equal(t, []indexing.SearchDocument{{NoteID: "n1", Text: "oi"}}, queue.produced,
		"o laço indexa com indexing.Index")
}

func TestRunLoop_ErroDoBanco_DevolveOErroSemEsperar(t *testing.T) {
	falha := errors.New("banco fora")
	queue := &fakeVaultQueue{err: falha}
	espera := &fakeWait{stopAfter: 1}

	err := indexing.RunLoop(context.Background(), queue, espera.wait)

	require.ErrorIs(t, err, falha)
	assert.Zero(t, espera.calls)
}

func TestWaitInterval_ContextoCancelado_VoltaNaHoraComOErro(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inicio := time.Now()
	err := indexing.WaitInterval(time.Hour)(ctx)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(inicio), time.Second)
}

func TestWaitInterval_IntervaloPassa_VoltaSemErro(t *testing.T) {
	assert.NoError(t, indexing.WaitInterval(time.Millisecond)(context.Background()))
}
