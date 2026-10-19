package oauth

import (
	"sync"
	"time"
)

// cimdFailureMemory é o cache negativo do CIMD: lembra até quando cada client_id cuja
// busca de documento falhou fica recusado sem buscar de novo.
//
// ponytail: em memória, vale por réplica (cada réplica busca uma vez por client_id
// falho); a alternativa é uma tabela, se várias réplicas precisarem concordar.
type cimdFailureMemory struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func newCIMDFailureMemory() *cimdFailureMemory {
	return &cimdFailureMemory{until: map[string]time.Time{}}
}

// remember recusa clientID até until e descarta as entradas já vencidas em now.
func (m *cimdFailureMemory) remember(clientID string, until, now time.Time) {
}

// failedUntil diz até quando clientID segue recusado; ok falso se não há falha vigente.
func (m *cimdFailureMemory) failedUntil(clientID string, now time.Time) (until time.Time, ok bool) {
	return time.Time{}, false
}
