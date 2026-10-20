package oauth

import (
	"context"
	"errors"
	"time"
)

// Client é um cliente OAuth registrado, por Client ID Metadata Document ou por DCR.
type Client struct {
	ID           string
	Name         string
	RedirectURIs []string
}

// AuthorizationRequest é o Pedido de autorização: a solicitação de um cliente para agir
// em nome de uma conta. O ID funciona como portador (ADR-0040): qualquer conta logada que
// o apresente aprova ou recusa o pedido antes de ExpiresAt.
type AuthorizationRequest struct {
	ID            string
	ClientID      string
	RedirectURI   string
	Scopes        []string
	Resource      string
	State         string
	CodeChallenge string
	CreatedAt     time.Time
	ExpiresAt     time.Time
	UserID        string
}

// Authorization é a Autorização ativa de uma conta para um cliente, com os escopos
// concedidos até agora.
type Authorization struct {
	ID        string
	UserID    string
	ClientID  string
	Scopes    []string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Revoked é preenchido por FindToken, RedeemCode e no reuso de
	// ConsumeRefreshToken, que devolvem a Autorização mesmo revogada.
	Revoked bool
}

// Tipos de token guardados em oauth_tokens.
const (
	TokenKindAccess  = "access"
	TokenKindRefresh = "refresh"
)

// IssuedToken é um token opaco emitido para uma Autorização, guardado só pelo hash. Leva
// os escopos do pedido que o originou, não a soma da Autorização.
type IssuedToken struct {
	Hash            string
	AuthorizationID string
	Kind            string
	Resource        string
	Scopes          []string
	ExpiresAt       time.Time
	// Revoked é preenchido por FindToken, que devolve o token mesmo revogado.
	Revoked bool
}

// Approval registra a aprovação de um pedido: o código emitido e a conta que aprovou.
// A Autorização passa a ter os escopos do pedido somados aos que já tinha.
type Approval struct {
	RequestID     string
	UserID        string
	CodeHash      string
	CodeExpiresAt time.Time
	DecidedAt     time.Time
}

// Denial registra a recusa de um pedido pela conta.
type Denial struct {
	RequestID string
	UserID    string
	DecidedAt time.Time
}

// RedeemedCode é o resultado de consumir um código: o pedido que o originou e a
// Autorização gravada nele na aprovação.
type RedeemedCode struct {
	Request       AuthorizationRequest
	Authorization Authorization
}

// Erros do repositório.
var (
	ErrClientNotFound        = errors.New("cliente oauth não encontrado")
	ErrRequestNotFound       = errors.New("pedido de autorização inexistente, expirado ou já decidido")
	ErrAuthorizationNotFound = errors.New("autorização ativa não encontrada")
	ErrCodeNotFound          = errors.New("código de autorização inexistente ou expirado")
	ErrCodeAlreadyUsed       = errors.New("código de autorização já usado")
	ErrTokenNotFound         = errors.New("token oauth inexistente, expirado ou revogado")
	ErrRefreshReused         = errors.New("refresh token oauth já rotacionado")
)

// Repository é a porta de persistência do authorization server. Todas as tabelas são
// globais (ADR-0034): a implementação não usa ator nem RLS.
type Repository interface {
	GetClient(ctx context.Context, id string) (Client, error)
	CreateRequest(ctx context.Context, req AuthorizationRequest) error
	// GetPendingRequest devolve ErrRequestNotFound se o pedido não existe, expirou em
	// relação a now ou já foi decidido.
	GetPendingRequest(ctx context.Context, id string, now time.Time) (AuthorizationRequest, error)
	// ApproveRequest marca o pedido como decidido, grava o hash do código, cria a
	// Autorização ativa da conta para o cliente ou amplia os escopos dela com os do
	// pedido, grava no pedido o ID dessa Autorização e audita oauth.authorize com
	// sucesso, tudo numa transação. Devolve ErrRequestNotFound se o pedido já não
	// estiver pendente.
	ApproveRequest(ctx context.Context, approval Approval) (AuthorizationRequest, error)
	// DenyRequest marca o pedido como decidido e audita oauth.authorize com falha, na
	// mesma transação. Devolve ErrRequestNotFound se o pedido já não estiver pendente.
	DenyRequest(ctx context.Context, denial Denial) (AuthorizationRequest, error)
	// ActiveAuthorization devolve ErrAuthorizationNotFound quando não há Autorização
	// ativa da conta para o cliente.
	ActiveAuthorization(ctx context.Context, userID, clientID string) (Authorization, error)
	// RedeemCode consome o código de uso único e lê, com a trava, a Autorização gravada
	// no pedido, não a ativa da conta para o cliente. Os erros, por caso:
	//   - ErrCodeNotFound: o hash não existe ou expirou sem uso em relação a now;
	//   - ErrAuthorizationNotFound, junto com o RedeemedCode: a Autorização do pedido
	//     foi revogada; o consumo fica gravado mesmo assim;
	//   - ErrCodeAlreadyUsed, junto com o RedeemedCode e a Autorização do pedido mesmo
	//     revogada: o código já foi consumido. Numa transação, o pedido fica marcado
	//     como reusado (guardando o primeiro reuso em now) e os tokens daquela
	//     Autorização são revogados com a trava dela (RFC 9700, 4.4.2).
	RedeemCode(ctx context.Context, codeHash string, now time.Time, reuseWindow time.Duration) (RedeemedCode, error)
	// SaveCodeTokens trava a Autorização dos tokens e grava o par emitido pela troca do
	// código do pedido requestID, numa transação. Sem gravar nada, devolve
	// ErrAuthorizationNotFound se a Autorização foi revogada e ErrCodeAlreadyUsed se o
	// código do pedido foi reapresentado depois do consumo, ou se o pedido não é dessa
	// Autorização.
	SaveCodeTokens(ctx context.Context, requestID string, tokens []IssuedToken) error
	// ConsumeRefreshToken marca o refresh token como rotacionado e lê a Autorização
	// ativa na mesma transação. Os erros, por caso:
	//   - ErrTokenNotFound: o hash não é de um refresh válido (inexistente, expirado em
	//     relação a now ou revogado, mesmo já rotacionado);
	//   - ErrRefreshReused, junto com o TokenGrant: o refresh já tinha sido rotacionado e
	//     não foi revogado; o grant traz a Autorização dona mesmo revogada, como
	//     FindToken, para a auditoria ter ator e alvo;
	//   - ErrAuthorizationNotFound: o refresh era vigente, mas a Autorização dele foi
	//     revogada; a rotação fica gravada mesmo assim.
	ConsumeRefreshToken(ctx context.Context, tokenHash string, now time.Time) (TokenGrant, error)
	// SaveRefreshedTokens trava a Autorização dos tokens, grava o novo par e audita o
	// evento, numa transação. Sem gravar nada, devolve ErrAuthorizationNotFound se a
	// Autorização foi revogada e ErrTokenNotFound se o refresh pai, pelo parentHash, foi
	// revogado depois do consumo.
	SaveRefreshedTokens(ctx context.Context, parentHash string, tokens []IssuedToken, event AuditEvent) error
	// RevokeAuthorizationTokensAudited trava a Autorização, revoga os tokens dela e audita
	// o evento, numa transação. triggerHash é o token que disparou a revogação: já com a
	// trava, ele precisa existir, ser da Autorização e não estar revogado e, com
	// ExpiredTriggerIgnored, não ter expirado em event.OccurredAt. Senão não revoga nem
	// audita e devolve nil. Devolve ErrAuthorizationNotFound se a Autorização não existe.
	RevokeAuthorizationTokensAudited(ctx context.Context, authorizationID, triggerHash string, expiry TriggerExpiry, event AuditEvent) error
	// FindToken devolve o dono de um token de qualquer tipo, mesmo revogado ou expirado,
	// ou ErrTokenNotFound.
	FindToken(ctx context.Context, tokenHash string) (TokenGrant, error)
	// ListActiveAuthorizations devolve as Autorizações ativas da conta com o nome do
	// cliente, da mais recente para a mais antiga.
	ListActiveAuthorizations(ctx context.Context, userID string) ([]AuthorizationSummary, error)
	// RevokeAuthorization trava a Autorização, confere que ela é da conta e segue ativa
	// antes de escrever, marca-a como revogada, revoga os tokens dela e audita o evento
	// com event.ClientID preenchido pela Autorização, numa transação. Devolve
	// ErrAuthorizationNotFound se ela não existe, já foi revogada ou é de outra conta.
	RevokeAuthorization(ctx context.Context, userID, authorizationID string, event AuditEvent) error
}

// TriggerExpiry diz se o token que disparou a revogação auditada ainda revoga depois de
// expirar.
type TriggerExpiry int

const (
	// ExpiredTriggerRevokes revoga mesmo com o gatilho expirado, como no reuso de refresh.
	ExpiredTriggerRevokes TriggerExpiry = iota
	// ExpiredTriggerIgnored não revoga com o gatilho expirado, como no /oauth/revoke.
	ExpiredTriggerIgnored
)

// TokenGrant liga um token emitido à Autorização que o originou.
type TokenGrant struct {
	Token         IssuedToken
	Authorization Authorization
}

// AuthorizationSummary é a Autorização ativa com o nome do cliente, para a conta listar.
type AuthorizationSummary struct {
	Authorization
	ClientName string
}

// Ações auditadas pelo authorization server além da decisão do pedido.
const (
	AuditActionRefresh      = "oauth.refresh"
	AuditActionRefreshReuse = "oauth.refresh_reuse"
	AuditActionRevoke       = "oauth.revoke"
	AuditActionCodeReuse    = "oauth.code_reuse"
)

// Resultados de um evento auditado.
const (
	AuditResultSuccess = "success"
	AuditResultFailure = "failure"
)

// AuditEvent é um evento do authorization server para o registro de operações críticas
// (ADR-0022), sempre sem tenant.
type AuditEvent struct {
	Action     string
	Result     string
	UserID     string
	ClientID   string
	OccurredAt time.Time
}
