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
// maxCIMDFailures limita o cache negativo, que qualquer pedido sem login consegue
// alimentar com client_ids inventados.
const maxCIMDFailures = 1000

type cimdFailureMemory struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func newCIMDFailureMemory() *cimdFailureMemory {
	return &cimdFailureMemory{until: map[string]time.Time{}}
}

// remember recusa clientID até until. Cheia, descarta as entradas vencidas em now e,
// se mesmo assim não houver lugar, não lembra a nova: o pedido continua recusado, só a
// próxima tentativa volta a buscar. Um until que não passa de now não é guardado.
func (m *cimdFailureMemory) remember(clientID string, until, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !now.Before(until) {
		return
	}
	if len(m.until) >= maxCIMDFailures {
		m.dropExpired(now)
	}
	if len(m.until) < maxCIMDFailures {
		m.until[clientID] = until
	}
}

// dropExpired descarta as falhas já vencidas em now; quem chama segura o mutex.
func (m *cimdFailureMemory) dropExpired(now time.Time) {
	for id, expiry := range m.until {
		if !now.Before(expiry) {
			delete(m.until, id)
		}
	}
}

// failedUntil diz até quando clientID segue recusado; ok falso se não há falha vigente.
func (m *cimdFailureMemory) failedUntil(clientID string, now time.Time) (until time.Time, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	until, ok = m.until[clientID]
	return until, ok && now.Before(until)
}
