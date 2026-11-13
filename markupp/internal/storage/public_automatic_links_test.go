package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/notes"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/storagetest"
)

// O cofre publicado tem alfa e gama, que citam a beta pelo título; o par gama e beta
// está recusado, então só a menção de alfa vira ligação, e a leitura pública a vê.
func TestGrafoPublico_MostraALigacaoPorMencaoESemARecusada(t *testing.T) {
	db := setupTestDB(t)
	repo := storage.NewPostgresNotesRepository(db, "simple")
	ctx := storagetest.SeedMember(t, db, identity.Actor{UserID: "ua", TenantID: "t1", Role: "owner"}, "va")
	for _, n := range []notes.Note{
		notaPublica("alfa", "Alfa.md", "va", "o texto de alfa cita a nota Beta"),
		notaPublica("beta", "Beta.md", "va", "a nota beta"),
		notaPublica("gama", "Gama.md", "va", "gama também cita Beta"),
	} {
		storagetest.SaveNote(ctx, t, repo, n)
	}
	_, err := db.Exec(context.Background(), `INSERT INTO link_refusals (vault_id, low_id, high_id, created_by) VALUES ('va', 'beta', 'gama', 'ua')`)
	require.NoError(t, err)
	publica(t, db, "p-va", "t1", "va", alvoCofre())
	indexaTodosOsCofres(t, db, "simple")
	alfa, beta := idPublicoDe(t, db, "alfa"), idPublicoDe(t, db, "beta")

	g, err := storage.NewPostgresPublicNotes(db, "simple").PublicGraph(context.Background(), alfa)

	require.NoError(t, err)
	assert.Equal(t, []publication.PublicGraphLink{{Source: indexOf(g, alfa), Target: indexOf(g, beta), Type: "mentioned"}}, g.Links)
}

func indexOf(g publication.PublicGraph, publicID string) int {
	for i, n := range g.Nodes {
		if n.PublicID == publicID {
			return i
		}
	}
	return -1
}
