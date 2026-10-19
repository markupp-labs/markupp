package notes

import (
	"time"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/indexing"
)

// LinkedNote é a projeção de uma nota no grafo de ligações, sem o conteúdo.
type LinkedNote struct {
	ID        string    `json:"id"`
	VaultID   string    `json:"vault_id"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteSummary é a nota citada em outro recurso: id, título e caminho.
type NoteSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Summarize resume a nota de id e caminho notePath, com o título que vem do caminho.
func Summarize(id, notePath string) NoteSummary {
	return NoteSummary{ID: id, Title: indexing.Title(notePath), Path: notePath}
}

// Tipos de ligação.
const (
	LinkExplicit = "explicit"
	LinkEmbed    = "embed"
	LinkTyped    = "typed"
	LinkInferred = "inferred"
)

// Situações de resolução de uma ligação.
const (
	LinkResolved   = "resolved"
	LinkUnresolved = "unresolved"
)

// Link é uma ligação que Source faz citando CitedTitle, como escrito na nota, para
// Target. A ligação ambígua aparece uma vez por nota com o título, e a não resolvida
// uma vez, com Target nil e o palpite do segundo estágio em InferredTarget, quando
// há. Predicate é a chave do frontmatter da ligação tipada, nil nas outras.
type Link struct {
	Source         NoteSummary  `json:"source"`
	Target         *NoteSummary `json:"target"`
	CitedTitle     string       `json:"cited_title"`
	Type           string       `json:"type"`
	Predicate      *string      `json:"predicate"`
	InferredTarget *NoteSummary `json:"inferred_target"`
}

// Page é a página pedida de uma lista: quantos itens pular e quantos devolver.
type Page struct {
	Offset int
	Limit  int
}

// LinkFilter recorta uma lista de ligações pelo tipo, vazio para todos.
type LinkFilter struct {
	Type string
	Page
}

// VaultLinkQuery recorta as ligações do cofre também pela resolução, vazia para as
// duas.
type VaultLinkQuery struct {
	Status string
	LinkFilter
}

// Sentidos de uma menção não ligada em relação à nota lida.
const (
	// MentionIncoming é a outra nota citando um título da nota lida.
	MentionIncoming = "incoming"
	// MentionOutgoing é a nota lida citando um título da outra.
	MentionOutgoing = "outgoing"
)

// UnlinkedMention é a menção não ligada entre a nota lida e a outra, que não se
// ligam em sentido algum: o título citado, na forma normalizada sem caixa, o
// sentido e a probabilidade a priori de o título apontar para a nota citada.
type UnlinkedMention struct {
	LinkedNote
	Title     string  `json:"title"`
	Direction string  `json:"direction"`
	Prior     float64 `json:"prior"`
}

// LinkSuggestion é a nota que a nota lida poderia ligar: a pontuação RRF que ordena a
// lista e os dois sinais fundidos nela, a alocação de recursos na vizinhança de dois
// saltos e o cosseno tf-idf; zero quando a nota não aparece na lista do sinal.
type LinkSuggestion struct {
	LinkedNote
	Score              float64 `json:"score"`
	ResourceAllocation float64 `json:"resource_allocation"`
	Cosine             float64 `json:"cosine"`
}

// CommunityMember é a nota que pertence a uma comunidade do cofre, com o id e o
// número de notas da comunidade.
type CommunityMember struct {
	LinkedNote
	CommunityID   int64 `json:"community_id"`
	CommunitySize int   `json:"community_size"`
}
