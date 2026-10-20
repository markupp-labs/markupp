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
// Autorização ativa da conta para o cliente.
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
	// pedido, e audita oauth.authorize com sucesso, tudo numa transação. Devolve
	// ErrRequestNotFound se o pedido já não estiver pendente.
	ApproveRequest(ctx context.Context, approval Approval) (AuthorizationRequest, error)
	// DenyRequest marca o pedido como decidido e audita oauth.authorize com falha, na
	// mesma transação. Devolve ErrRequestNotFound se o pedido já não estiver pendente.
	DenyRequest(ctx context.Context, denial Denial) (AuthorizationRequest, error)
	// ActiveAuthorization devolve ErrAuthorizationNotFound quando não há Autorização
	// ativa da conta para o cliente.
	ActiveAuthorization(ctx context.Context, userID, clientID string) (Authorization, error)
	// RedeemCode consome o código de uso único. Devolve ErrCodeNotFound se o hash não
	// existe ou expirou em relação a now, e ErrCodeAlreadyUsed junto com o RedeemedCode
	// do pedido original quando o código já foi consumido.
	RedeemCode(ctx context.Context, codeHash string, now time.Time) (RedeemedCode, error)
	SaveTokens(ctx context.Context, tokens []IssuedToken) error
	// RevokeAuthorizationTokens revoga todos os tokens da Autorização.
	RevokeAuthorizationTokens(ctx context.Context, authorizationID string) error
}
