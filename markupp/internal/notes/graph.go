package notes

import "time"

// LinkedNote é a projeção de uma nota no grafo de ligações, sem o conteúdo.
type LinkedNote struct {
	ID        string    `json:"id"`
	VaultID   string    `json:"vault_id"`
	Path      string    `json:"path"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Backlink é uma nota que liga a outra, com os predicados das ligações tipadas que
// ela faz para a outra, vazio quando ela liga só pelo corpo, e Inferred quando ela
// liga só por ligação inferida.
type Backlink struct {
	LinkedNote
	Predicates []string `json:"predicates"`
	Inferred   bool     `json:"inferred"`
}

// Neighbor é uma nota ligada a outra, com o sentido da ligação: Incoming quando ela
// liga à outra, Outgoing quando a outra liga a ela; as duas coisas podem valer. Os
// predicados das ligações tipadas vêm separados pelo mesmo sentido, e Inferred diz
// que as duas se ligam só por ligação inferida.
type Neighbor struct {
	LinkedNote
	Incoming           bool     `json:"incoming"`
	Outgoing           bool     `json:"outgoing"`
	IncomingPredicates []string `json:"incoming_predicates"`
	OutgoingPredicates []string `json:"outgoing_predicates"`
	Inferred           bool     `json:"inferred"`
}

// UnresolvedLink é a ligação da nota NoteID, de caminho Path, para um título que
// nenhuma nota do cofre tem, como escrito na nota.
type UnresolvedLink struct {
	NoteID string `json:"note_id"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
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
