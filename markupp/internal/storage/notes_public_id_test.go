package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
)

func TestNotaDoMembro_LeituraEListagemTrazemOIDPublico(t *testing.T) {
	db, ctx, repo := cofreComPastas(t)
	publica(t, db, "p-docs", "t1", "va", alvoPasta("docs"))
	idPublico := idPublicoDe(t, db, "a")

	lida, err := repo.GetNoteByID(ctx, "a")
	require.NoError(t, err)
	privada, err := repo.GetNoteByID(ctx, "d")
	require.NoError(t, err)
	lista, err := repo.ListVaultNotes(ctx, "va", notes.VaultNotesQuery{Sort: notes.SortByUpdated, Limit: 10})
	require.NoError(t, err)

	require.NotNil(t, lida.PublicID)
	assert.Equal(t, idPublico, *lida.PublicID)
	assert.Nil(t, privada.PublicID)
	publicas := map[string]bool{}
	for _, n := range lista {
		publicas[n.ID] = n.PublicID != nil
	}
	assert.Equal(t, map[string]bool{"a": true, "b": true, "c": false, "d": false}, publicas)
}

func TestNotaDoMembro_OrfasTambemTrazemOIDPublico(t *testing.T) {
	db, ctx, _ := cofreComPastas(t)
	publica(t, db, "p-cofre", "t1", "va", alvoCofre())
	indexaTodosOsCofres(t, db, "simple")
	orfa := true

	lista, err := storage.NewPostgresNotesRepository(db, "simple").ListVaultNotes(ctx, "va", notes.VaultNotesQuery{Orphan: &orfa, Sort: notes.SortByUpdated, Limit: 10})

	require.NoError(t, err)
	require.NotEmpty(t, lista)
	for _, n := range lista {
		assert.NotNil(t, n.PublicID, n.ID)
	}
}
