package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// A pasta public está publicada e a nota Index cita [[private/Plan]], que existe mas
// é privada. A leitura pública não enxerga private/Plan.md, e mesmo assim o link não
// pode cair no fallback por título e ir para public/Plan.md: fica sem destino.
func TestNotaPublica_LinkParaCaminhoPrivadoNaoResolveParaOutraNota(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "va")
	storagetest.SaveNote(ctx, t, repo, notaPublica("index", "public/Index.md", "va", "veja [[private/Plan]]"))
	storagetest.SaveNote(ctx, t, repo, notaPublica("plano-publico", "public/Plan.md", "va", "plano publico"))
	storagetest.SaveNote(ctx, t, repo, notaPublica("plano-privado", "private/Plan.md", "va", "plano privado"))
	indexaTodosOsCofres(t, db, "simple")
	publica(t, db, "p-public", "t1", "va", alvoPasta("public"))
	index := idPublicoDe(t, db, "index")
	leitura := storage.NewPostgresPublicNotes(db, "simple")

	nota, err := leitura.PublicNote(context.Background(), index)
	require.NoError(t, err)
	g, err := leitura.PublicGraph(context.Background(), index)
	require.NoError(t, err)

	assert.Equal(t, []publication.PublicLink{{CitedTitle: "Plan", Kind: "explicit"}}, nota.Links)
	assert.Empty(t, g.Links)
}
