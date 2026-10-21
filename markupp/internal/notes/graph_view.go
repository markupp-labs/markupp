package notes

// Profundidades aceitas no grafo local de uma nota, em saltos a partir dela.
const (
	MinGraphDepth     = 1
	MaxGraphDepth     = 3
	DefaultGraphDepth = 1
)

// GraphFilter recorta o grafo que o cliente desenha: HideOrphans tira as notas de
// grau zero, HideUnresolved os nós sem nota (títulos não resolvidos e anexos),
// HideInferred as ligações inferidas, e Community, quando não nil, fica só com as
// notas da comunidade.
type GraphFilter struct {
	HideOrphans    bool
	HideUnresolved bool
	HideInferred   bool
	Community      *int64
}

// GraphNote é a nota que vira nó, com a organização derivada dela.
type GraphNote struct {
	NoteSummary
	Rank NoteRank
}

// GraphEdge é uma ligação resolvida entre duas notas.
type GraphEdge struct {
	SourceID  string
	TargetID  string
	Type      string
	Predicate *string
}

// GraphCitation é uma ligação não resolvida da nota NoteID, que cita CitedTitle,
// de chave TitleKey.
type GraphCitation struct {
	NoteID     string
	TitleKey   string
	CitedTitle string
	Type       string
	Predicate  *string
}

// GraphNode é um nó do grafo: uma nota, ou o título não resolvido, com ID, Path e a
// organização derivada nil e Unresolved verdadeiro.
type GraphNode struct {
	ID         *string  `json:"id"`
	Title      string   `json:"title"`
	Path       *string  `json:"path"`
	Degree     *int     `json:"degree"`
	PageRank   *float64 `json:"pagerank"`
	Community  *int64   `json:"community"`
	Unresolved bool     `json:"unresolved"`
}

// GraphLink é uma ligação entre os nós de índices Source e Target em Graph.Nodes.
type GraphLink struct {
	Source    int     `json:"source"`
	Target    int     `json:"target"`
	Type      string  `json:"type"`
	Predicate *string `json:"predicate"`
}

// Graph é o grafo para desenhar, com as ligações apontando para os nós por índice.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// graphBuilder monta o Graph lembrando o índice de cada nota e de cada título não
// resolvido.
type graphBuilder struct {
	graph      Graph
	noteIndex  map[string]int
	titleIndex map[string]int
}

// BuildGraph monta o grafo com um nó por nota, na ordem recebida, seguido de um nó
// por chave de título não resolvido, com a primeira grafia citada. Ligação ou citação
// de nota que não está entre os nós sai.
func BuildGraph(graphNotes []GraphNote, edges []GraphEdge, citations []GraphCitation) Graph {
	b := graphBuilder{graph: Graph{Nodes: []GraphNode{}, Links: []GraphLink{}}, noteIndex: map[string]int{}, titleIndex: map[string]int{}}
	for _, n := range graphNotes {
		b.addNote(n)
	}
	for _, e := range edges {
		b.addEdge(e)
	}
	for _, c := range citations {
		b.addCitation(c)
	}
	return b.graph
}

func (b *graphBuilder) addNote(n GraphNote) {
	b.noteIndex[n.ID] = len(b.graph.Nodes)
	b.graph.Nodes = append(b.graph.Nodes, GraphNode{
		ID: &n.ID, Title: n.Title, Path: &n.Path, Degree: n.Rank.Degree, PageRank: n.Rank.PageRank, Community: n.Rank.Community,
	})
}

func (b *graphBuilder) addEdge(e GraphEdge) {
	source, okSource := b.noteIndex[e.SourceID]
	target, okTarget := b.noteIndex[e.TargetID]
	if !okSource || !okTarget {
		return
	}
	b.graph.Links = append(b.graph.Links, GraphLink{Source: source, Target: target, Type: e.Type, Predicate: e.Predicate})
}

func (b *graphBuilder) addCitation(c GraphCitation) {
	source, ok := b.noteIndex[c.NoteID]
	if !ok {
		return
	}
	target, ok := b.titleIndex[c.TitleKey]
	if !ok {
		target = len(b.graph.Nodes)
		b.titleIndex[c.TitleKey] = target
		b.graph.Nodes = append(b.graph.Nodes, GraphNode{Title: c.CitedTitle, Unresolved: true})
	}
	b.graph.Links = append(b.graph.Links, GraphLink{Source: source, Target: target, Type: c.Type, Predicate: c.Predicate})
}
