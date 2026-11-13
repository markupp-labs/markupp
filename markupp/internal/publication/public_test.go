package publication_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/publication"
)

func TestTitleSlug_MinusculasSemAcentoEHifensColapsados(t *testing.T) {
	casos := map[string]string{
		"Guia de Deploy":          "guia-de-deploy",
		"Ação & Reação: 2ª parte": "acao-reacao-2-parte",
		"  --Olá,   mundo!--  ":   "ola-mundo",
		"日本語":                     "nota",
		"":                        "nota",
		"Über_Straße":             "uber-stra-e",
	}
	for titulo, esperado := range casos {
		assert.Equal(t, esperado, publication.TitleSlug(titulo), "titulo %q", titulo)
	}
}

func TestTitleSlug_NoMaximoSessentaCaracteresSemHifenNoFim(t *testing.T) {
	slug := publication.TitleSlug(strings.Repeat("a", 59) + " bcd")

	assert.Equal(t, strings.Repeat("a", 59), slug)
	assert.LessOrEqual(t, len(publication.TitleSlug(strings.Repeat("palavra ", 30))), 60)
}

func TestValidPublicID_DozeCaracteresBase62(t *testing.T) {
	for id, valido := range map[string]bool{
		"Ab3dEf6hIj9k": true, "Ab3dEf6hIj9": false, "Ab3dEf6hIj9kL": false,
		"Ab3dEf6hIj9-": false, "../../etc/pa": false, "": false,
	} {
		assert.Equal(t, valido, publication.ValidPublicID(id), "id %q", id)
	}
}

func TestScopeTitle_NotaPastaECofre(t *testing.T) {
	assert.Equal(t, "Guia", publication.ScopeTitle(publication.Scope{Kind: publication.KindNote}, "Guia"))
	assert.Equal(t, "deploy", publication.ScopeTitle(publication.Scope{Kind: publication.KindFolder, FolderPath: "docs/deploy"}, "Guia"))
	assert.Equal(t, "Manual", publication.ScopeTitle(publication.Scope{Kind: publication.KindVault, VaultName: "Manual"}, "Guia"))
}

func TestRelativePath_RelativoAPastaPublicada(t *testing.T) {
	assert.Equal(t, "x/b.md", publication.RelativePath(publication.Scope{Kind: publication.KindFolder, FolderPath: "docs"}, "docs/x/b.md"))
	assert.Equal(t, "docs/x/b.md", publication.RelativePath(publication.Scope{Kind: publication.KindVault}, "docs/x/b.md"))
	assert.Equal(t, "b.md", publication.RelativePath(publication.Scope{Kind: publication.KindNote}, "docs/x/b.md"), "a nota avulsa nao mostra as pastas do cofre")
}
