package publication

import (
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// maxSlugLength é o maior title_slug, em bytes ASCII.
const maxSlugLength = 60

// emptySlug é o title_slug do título que não deixa nenhum caractere.
const emptySlug = "nota"

// publicIDPattern é o mesmo CHECK de public_notes.public_id.
var publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{12}$`)

// Scope é a publicação mais ampla que cobre a nota pública lida. Fica no servidor: o
// id interno da nota e o da publicação nunca saem nas rotas públicas.
type Scope struct {
	NoteID        string
	VaultID       string
	PublicationID string
	Kind          string
	FolderPath    string
	VaultName     string
	Indexable     bool
}

// PublicNote é a nota lida sem conta, pelo id público.
type PublicNote struct {
	PublicID    string
	Title       string
	Content     string
	UpdatedAt   time.Time
	Indexable   bool
	Publication PublicScope
	Links       []PublicLink
}

// PublicScope é a publicação da nota como a página pública mostra. RootPublicID é a
// própria nota na publicação de nota, e nil na de pasta e na de cofre.
type PublicScope struct {
	Kind         string
	Title        string
	RootPublicID *string
}

// PublicLink é uma ligação da nota. PublicID é nil quando o alvo não é público.
type PublicLink struct {
	CitedTitle string
	Kind       string
	PublicID   *string
}

// PublicTreeItem é uma nota da mesma publicação, com o caminho relativo à pasta publicada.
type PublicTreeItem struct {
	PublicID string
	Title    string
	Path     string
}

// PublicSearchHit é um resultado da busca na publicação, com o trecho marcado por
// U+0002 e U+0003.
type PublicSearchHit struct {
	PublicID string
	Title    string
	Snippet  string
	Rank     float64
}

// PublicGraph é o grafo da publicação, com os nós pelo id público.
type PublicGraph struct {
	Nodes []PublicGraphNode
	Links []PublicGraphLink
}

// PublicGraphNode é uma nota da publicação com a organização derivada dela.
type PublicGraphNode struct {
	PublicID  string
	Title     string
	Degree    *int
	PageRank  *float64
	Community *int64
}

// PublicGraphLink liga os nós de índices Source e Target em PublicGraph.Nodes.
type PublicGraphLink struct {
	Source int
	Target int
	Type   string
}

// ValidPublicID diz se id tem o formato do id público, para recusar sem ir ao banco.
func ValidPublicID(id string) bool {
	return publicIDPattern.MatchString(id)
}

// TitleSlug é o enfeite do endereço público: minúsculas sem acento, o que não é letra
// ou dígito ASCII vira hífen, hífens colapsados, até 60 caracteres, e "nota" se vazio.
func TitleSlug(title string) string {
	var b strings.Builder
	for _, r := range withoutMarks(strings.ToLower(title)) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else if !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugLength {
		slug = strings.TrimRight(slug[:maxSlugLength], "-")
	}
	if slug == "" {
		return emptySlug
	}
	return slug
}

// withoutMarks tira os acentos: decompõe e descarta as marcas combinantes.
func withoutMarks(s string) string {
	out, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), s)
	if err != nil {
		return s
	}
	return out
}

// ScopeTitle é o título da publicação: o da nota, o último trecho da pasta ou o nome
// do cofre.
func ScopeTitle(s Scope, noteTitle string) string {
	switch s.Kind {
	case KindFolder:
		return path.Base(s.FolderPath)
	case KindVault:
		return s.VaultName
	}
	return noteTitle
}

// RelativePath é o caminho da nota relativo à pasta publicada. A nota avulsa mostra só
// o nome, sem as pastas do cofre.
func RelativePath(s Scope, notePath string) string {
	switch s.Kind {
	case KindFolder:
		return strings.TrimPrefix(notePath, s.FolderPath+"/")
	case KindVault:
		return notePath
	}
	return path.Base(notePath)
}
