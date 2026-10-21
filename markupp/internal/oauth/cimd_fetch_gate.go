package oauth

import (
	"errors"
	"fmt"

	"golang.org/x/sync/singleflight"
)

// maxCIMDFetches limita as buscas de documento CIMD em andamento ao mesmo tempo.
//
// ponytail: em memória, vale por réplica (N réplicas fazem até N*8 buscas para fora e
// cada uma junta só os pedidos que recebe); um limite global pediria coordenação entre
// as réplicas.
const maxCIMDFetches = 8

// errCIMDFetchBusy indica que a busca nem saiu porque todas as vagas estavam ocupadas.
var errCIMDFetchBusy = errors.New("oauth: buscas de documento cimd no limite")

// cimdFetchGate junta numa só as buscas simultâneas do mesmo client_id e recusa, sem
// buscar, quando já há maxCIMDFetches buscas para fora em andamento.
type cimdFetchGate struct {
	flights singleflight.Group
	slots   chan struct{}
}

func newCIMDFetchGate() *cimdFetchGate {
	return &cimdFetchGate{slots: make(chan struct{}, maxCIMDFetches)}
}

// fetch roda fetchDocument uma vez por clientID em andamento e entrega o mesmo resultado
// a quem chegou durante a busca. Sem vaga, devolve errCIMDFetchBusy embrulhado.
func (g *cimdFetchGate) fetch(clientID string, fetchDocument func() (ClientMetadata, error)) (ClientMetadata, error) {
	result, err, _ := g.flights.Do(clientID, func() (any, error) {
		select {
		case g.slots <- struct{}{}:
		default:
			return ClientMetadata{}, fmt.Errorf("%w: %d em andamento ao buscar %s; esperada uma vaga livre", errCIMDFetchBusy, maxCIMDFetches, ClientValue(clientID))
		}
		defer func() { <-g.slots }()
		return fetchDocument()
	})
	metadata, _ := result.(ClientMetadata)
	return metadata, err
}
