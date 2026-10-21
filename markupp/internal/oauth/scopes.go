// Package oauth reúne o vocabulário do authorization server OAuth 2.1 (ADR-0046).
package oauth

// Escopos que um agente pode pedir ao authorization server.
const (
	ScopeNotesRead  = "notas:leitura"
	ScopeNotesWrite = "notas:escrita"
	ScopeAdmin      = "administracao"
)

// SupportedScopes devolve os escopos suportados em ordem fixa, numa fatia nova a cada chamada.
func SupportedScopes() []string {
	return []string{ScopeNotesRead, ScopeNotesWrite, ScopeAdmin}
}
