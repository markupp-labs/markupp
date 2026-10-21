package indexing_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
)

func TestIndex_NotasMudadas_DevolveUmDocumentoPorNotaComTituloEConteudo(t *testing.T) {
	cs := indexing.Changeset{TenantID: "t1", VaultID: "va", Notes: []indexing.NoteChange{
		{NoteID: "n1", Path: "atas/Reuniao Semanal.md", Content: "reuniao com o cliente"},
		{NoteID: "n2", Path: "decisao.md", Content: "decisao tecnica"},
	}}

	docs := indexing.Index(cs)

	assert.Equal(t, []indexing.IndexedNote{
		{Search: indexing.SearchDocument{NoteID: "n1", Title: "Reuniao Semanal", Text: "reuniao com o cliente"}, TitleKeys: []string{"reuniao semanal"}},
		{Search: indexing.SearchDocument{NoteID: "n2", Title: "decisao", Text: "decisao tecnica"}, TitleKeys: []string{"decisao"}},
	}, docs)
}

func TestTitle_UltimoSegmentoDoCaminhoSemExtensaoMd(t *testing.T) {
	casos := map[string]string{
		"nota.md":               "nota",
		"pasta/sub/Nota Boa.md": "Nota Boa",
		"versao.v2.md":          "versao.v2",
		"a.b.md":                "a.b",
		"sem-extensao":          "sem-extensao",
		"arquivo":               "arquivo",
		"Nota.MD":               "Nota",
		"Nota.Md":               "Nota",
		"pasta/.oculta.md":      ".oculta",
		"foto.png":              "foto.png",
		"pasta/doc.txt":         "doc.txt",
	}
	for caminho, titulo := range casos {
		assert.Equal(t, titulo, indexing.Title(caminho), "titulo do caminho %q", caminho)
	}
}

func TestIndex_SemNotas_DevolveVazio(t *testing.T) {
	assert.Empty(t, indexing.Index(indexing.Changeset{TenantID: "t1", VaultID: "va"}))
}

// fakeVaultQueue faz o papel do banco: falha nas primeiras chamadas com errs,
// depois devolve, chamada a chamada, se havia cofre pendente, e aplica a função de
// indexação recebida a um lote fixo para o teste conferir qual função chegou.
type fakeVaultQueue struct {
	errs     []error
	pending  []bool
	calls    int
	produced []indexing.IndexedNote
}

func (f *fakeVaultQueue) IndexNextVault(_ context.Context, index indexing.IndexFunc) (bool, error) {
	f.calls++
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return false, err
	}
	if len(f.pending) == 0 {
		return false, nil
	}
	found := f.pending[0]
	f.pending = f.pending[1:]
	if found {
		f.produced = index(indexing.Changeset{Notes: []indexing.NoteChange{{NoteID: "n1", Path: "oi.md", Content: "oi"}}})
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

	err := indexing.RunLoop(context.Background(), queue, espera.wait, slog.New(slog.DiscardHandler))

	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 4, queue.calls, "dois cofres, uma rodada vazia, espera, outra rodada vazia")
	assert.Equal(t, 2, espera.calls)
	assert.Equal(t, []indexing.IndexedNote{{Search: indexing.SearchDocument{NoteID: "n1", Title: "oi", Text: "oi"}, TitleKeys: []string{"oi"}}}, queue.produced,
		"o laço indexa com indexing.Index")
}

func TestRunLoop_ErroDoBanco_RegistraEEsperaAntesDeTentarDeNovo(t *testing.T) {
	falha := errors.New("banco fora")
	queue := &fakeVaultQueue{errs: []error{falha}, pending: []bool{true, false}}
	espera := &fakeWait{stopAfter: 2}
	var logs bytes.Buffer

	err := indexing.RunLoop(context.Background(), queue, espera.wait, slog.New(slog.NewJSONHandler(&logs, nil)))

	assert.ErrorIs(t, err, context.Canceled, "o laco so termina quando a espera termina")
	assert.Equal(t, 3, queue.calls, "erro, espera, cofre indexado, rodada vazia, espera")
	assert.Equal(t, 2, espera.calls, "o erro espera o intervalo, senao o laco martela o banco")
	assert.Contains(t, logs.String(), "banco fora")
}
