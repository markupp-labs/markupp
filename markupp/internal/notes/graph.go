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
// ela faz para a outra; vazio quando ela liga só pelo corpo.
type Backlink struct {
	LinkedNote
	Predicates []string `json:"predicates"`
}

// Neighbor é uma nota ligada a outra, com o sentido da ligação: Incoming quando ela
// liga à outra, Outgoing quando a outra liga a ela; as duas coisas podem valer. Os
// predicados das ligações tipadas vêm separados pelo mesmo sentido.
type Neighbor struct {
	LinkedNote
	Incoming           bool     `json:"incoming"`
	Outgoing           bool     `json:"outgoing"`
	IncomingPredicates []string `json:"incoming_predicates"`
	OutgoingPredicates []string `json:"outgoing_predicates"`
}

// UnresolvedLink é a ligação da nota NoteID, de caminho Path, para um título que
// nenhuma nota do cofre tem, como escrito na nota.
type UnresolvedLink struct {
	NoteID string `json:"note_id"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
}
