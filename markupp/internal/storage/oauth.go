package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/auth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

const auditActionOAuthAuthorize = "oauth.authorize"

// PostgresOAuthRepository persiste clientes, pedidos, Autorizações e tokens do
// authorization server OAuth. As tabelas são da instância (ADR-0039): sem ator nem RLS.
// Códigos, tokens e o ID do pedido ficam fora das mensagens de erro.
type PostgresOAuthRepository struct {
	pool *pgxpool.Pool
	q    *gen.Queries
}

var _ oauth.Repository = (*PostgresOAuthRepository)(nil)

// NewPostgresOAuthRepository monta o repositório OAuth sobre um pool aberto.
func NewPostgresOAuthRepository(pool *pgxpool.Pool) *PostgresOAuthRepository {
	return &PostgresOAuthRepository{pool: pool, q: gen.New(pool)}
}

// GetClient devolve o cliente registrado, ou oauth.ErrClientNotFound.
func (r *PostgresOAuthRepository) GetClient(ctx context.Context, id string) (oauth.Client, error) {
	row, err := r.q.GetOAuthClient(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Client{}, fmt.Errorf("%w: client_id %s", oauth.ErrClientNotFound, oauth.ClientValue(id))
	}
	if err != nil {
		return oauth.Client{}, fmt.Errorf("buscar cliente oauth %s: %w", oauth.ClientValue(id), err)
	}
	return oauth.Client{ID: row.ID, Name: row.Name, RedirectURIs: row.RedirectUris}, nil
}

// CreateRequest grava o Pedido de autorização ainda sem conta nem código.
func (r *PostgresOAuthRepository) CreateRequest(ctx context.Context, req oauth.AuthorizationRequest) error {
	err := r.q.CreateOAuthRequest(ctx, gen.CreateOAuthRequestParams{
		ID: req.ID, ClientID: req.ClientID, RedirectUri: req.RedirectURI, Scopes: req.Scopes,
		Resource: req.Resource, State: pgtype.Text{String: req.State, Valid: req.State != ""},
		CodeChallenge: req.CodeChallenge, CreatedAt: req.CreatedAt, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("gravar pedido de autorização do cliente %s: %w", oauth.ClientValue(req.ClientID), err)
	}
	return nil
}

// GetPendingRequest devolve o pedido ainda não decidido e não expirado em now.
func (r *PostgresOAuthRepository) GetPendingRequest(ctx context.Context, id string, now time.Time) (oauth.AuthorizationRequest, error) {
	row, err := r.q.GetPendingOAuthRequest(ctx, gen.GetPendingOAuthRequestParams{ID: id, Now: now})
	if err != nil {
		return oauth.AuthorizationRequest{}, requestErr("buscar pedido pendente", err)
	}
	return toAuthorizationRequest(row), nil
}

// ApproveRequest decide o pedido, cria ou amplia a Autorização e audita, numa transação.
// Recusa sem gravar nada a aprovação sem hash de código.
func (r *PostgresOAuthRepository) ApproveRequest(ctx context.Context, a oauth.Approval) (oauth.AuthorizationRequest, error) {
	if a.CodeHash == "" {
		return oauth.AuthorizationRequest{}, errors.New(`aprovar pedido de autorização: code_hash "" vazio, esperado o hash do código emitido`)
	}
	var row gen.OauthRequest
	err := inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		var err error
		row, err = approveInTx(ctx, qtx, a)
		return err
	})
	if err != nil {
		return oauth.AuthorizationRequest{}, err
	}
	return toAuthorizationRequest(row), nil
}

// approveInTx decide o pedido como aprovado, concede os escopos, liga o pedido à
// Autorização e audita o sucesso.
func approveInTx(ctx context.Context, qtx *gen.Queries, a oauth.Approval) (gen.OauthRequest, error) {
	row, err := qtx.ApproveOAuthRequest(ctx, gen.ApproveOAuthRequestParams{
		ID: a.RequestID, UserID: a.UserID, CodeHash: a.CodeHash,
		CodeExpiresAt: a.CodeExpiresAt, DecidedAt: a.DecidedAt,
	})
	if err != nil {
		return row, requestErr("aprovar pedido", err)
	}
	authorizationID, err := grantScopes(ctx, qtx, row, a.DecidedAt)
	if err != nil {
		return row, err
	}
	err = qtx.SetOAuthRequestAuthorization(ctx, gen.SetOAuthRequestAuthorizationParams{ID: row.ID, AuthorizationID: authorizationID})
	if err != nil {
		return row, fmt.Errorf("ligar o pedido aprovado à autorização %q: %w", authorizationID, err)
	}
	return row, auditAuthorize(ctx, qtx, row, "success", a.DecidedAt)
}

// DenyRequest decide o pedido como recusado e audita a falha, numa transação.
func (r *PostgresOAuthRepository) DenyRequest(ctx context.Context, d oauth.Denial) (oauth.AuthorizationRequest, error) {
	var row gen.OauthRequest
	err := inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		var err error
		row, err = qtx.DenyOAuthRequest(ctx, gen.DenyOAuthRequestParams{ID: d.RequestID, UserID: d.UserID, DecidedAt: d.DecidedAt})
		if err != nil {
			return requestErr("recusar pedido", err)
		}
		return auditAuthorize(ctx, qtx, row, "failure", d.DecidedAt)
	})
	if err != nil {
		return oauth.AuthorizationRequest{}, err
	}
	return toAuthorizationRequest(row), nil
}

// grantScopes cria a Autorização ativa da conta para o cliente com os escopos do pedido,
// ou amplia a existente com eles, num upsert atômico. Devolve o ID dela.
func grantScopes(ctx context.Context, qtx *gen.Queries, req gen.OauthRequest, at time.Time) (string, error) {
	id, err := qtx.UpsertOAuthAuthorization(ctx, gen.UpsertOAuthAuthorizationParams{
		ID: uuid.NewString(), UserID: req.UserID.String, ClientID: req.ClientID,
		Scopes: orderedScopes(req.Scopes), At: at, Supported: oauth.SupportedScopes(),
	})
	if err != nil {
		return "", fmt.Errorf("conceder escopos de %q para %s: %w", req.UserID.String, oauth.ClientValue(req.ClientID), err)
	}
	return id, nil
}

// orderedScopes devolve os escopos na ordem de oauth.SupportedScopes, sem repetir.
func orderedScopes(scopes []string) []string {
	return slices.DeleteFunc(oauth.SupportedScopes(), func(scope string) bool {
		return !slices.Contains(scopes, scope)
	})
}

// auditAuthorize registra a decisão do pedido como oauth.authorize, com o cliente como
// alvo e a conta que decidiu como ator.
func auditAuthorize(ctx context.Context, qtx *gen.Queries, req gen.OauthRequest, result string, at time.Time) error {
	return auditOAuth(ctx, qtx, oauth.AuditEvent{
		Action: auditActionOAuthAuthorize, Result: result, UserID: req.UserID.String,
		ClientID: req.ClientID, OccurredAt: at,
	})
}

// auditOAuth grava o evento do authorization server sem tenant, com a conta como ator e
// o cliente como alvo.
func auditOAuth(ctx context.Context, qtx *gen.Queries, e oauth.AuditEvent) error {
	err := qtx.CreateAuditEvent(ctx, toCreateAuditEventParams(auth.AuditEventRecord{
		ID: uuid.NewString(), ActorID: e.UserID, Action: e.Action,
		Target: e.ClientID, Result: e.Result, OccurredAt: e.OccurredAt,
	}))
	if err != nil {
		return fmt.Errorf("registrar auditoria %s (%s) para o cliente %s: %w", e.Action, e.Result, oauth.ClientValue(e.ClientID), err)
	}
	return nil
}

// ActiveAuthorization devolve a Autorização não revogada da conta para o cliente.
func (r *PostgresOAuthRepository) ActiveAuthorization(ctx context.Context, userID, clientID string) (oauth.Authorization, error) {
	return activeAuthorization(ctx, r.q, userID, clientID)
}

// activeAuthorization busca a Autorização ativa por q, que pode ser a de uma transação.
func activeAuthorization(ctx context.Context, q *gen.Queries, userID, clientID string) (oauth.Authorization, error) {
	row, err := q.GetActiveOAuthAuthorization(ctx, gen.GetActiveOAuthAuthorizationParams{UserID: userID, ClientID: clientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Authorization{}, fmt.Errorf("%w: usuário %q, cliente %s", oauth.ErrAuthorizationNotFound, userID, oauth.ClientValue(clientID))
	}
	if err != nil {
		return oauth.Authorization{}, fmt.Errorf("buscar autorização de %q para %s: %w", userID, oauth.ClientValue(clientID), err)
	}
	return toAuthorization(row), nil
}

// saveTokensInTx grava os tokens por qtx, parando no primeiro erro.
func saveTokensInTx(ctx context.Context, qtx *gen.Queries, tokens []oauth.IssuedToken) error {
	for _, t := range tokens {
		err := qtx.SaveOAuthToken(ctx, gen.SaveOAuthTokenParams{
			TokenHash: t.Hash, AuthorizationID: t.AuthorizationID, Kind: t.Kind,
			Resource: t.Resource, Scopes: t.Scopes, ExpiresAt: t.ExpiresAt,
		})
		if err != nil {
			return fmt.Errorf("gravar token %s da autorização %q: %w", t.Kind, t.AuthorizationID, err)
		}
	}
	return nil
}

// RevokeAuthorizationTokens trava a Autorização e revoga todos os tokens dela, numa
// transação.
func (r *PostgresOAuthRepository) RevokeAuthorizationTokens(ctx context.Context, authorizationID string) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		_, err := revokeTokens(ctx, qtx, authorizationID)
		return err
	})
}

// revokeTokens trava a Autorização e revoga os tokens dela por qtx, que precisa ser de
// uma transação para a trava valer até o commit. A trava serializa a revogação com a
// gravação do par rotacionado, que trava a mesma linha: sem ela, um par gravado em
// paralelo escaparia do UPDATE. Devolve a linha travada.
func revokeTokens(ctx context.Context, qtx *gen.Queries, authorizationID string) (gen.OauthAuthorization, error) {
	row, err := lockAuthorization(ctx, qtx, authorizationID)
	if err != nil {
		return row, err
	}
	return row, revokeLockedTokens(ctx, qtx, authorizationID)
}

// revokeLockedTokens revoga os tokens da Autorização que a transação de qtx já travou com
// lockAuthorization. É o único caminho do UPDATE que revoga tokens.
func revokeLockedTokens(ctx context.Context, qtx *gen.Queries, authorizationID string) error {
	if err := qtx.RevokeOAuthAuthorizationTokens(ctx, authorizationID); err != nil {
		return fmt.Errorf("revogar tokens da autorização %q: %w", authorizationID, err)
	}
	return nil
}

// requestErr traduz a ausência de linha para oauth.ErrRequestNotFound.
func requestErr(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.ErrRequestNotFound
	}
	return fmt.Errorf("%s de autorização: %w", op, err)
}

func toAuthorizationRequest(row gen.OauthRequest) oauth.AuthorizationRequest {
	return oauth.AuthorizationRequest{
		ID: row.ID, ClientID: row.ClientID, RedirectURI: row.RedirectUri, Scopes: row.Scopes,
		Resource: row.Resource, State: row.State.String, CodeChallenge: row.CodeChallenge,
		CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, UserID: row.UserID.String,
	}
}

func toAuthorization(row gen.OauthAuthorization) oauth.Authorization {
	return oauth.Authorization{
		ID: row.ID, UserID: row.UserID, ClientID: row.ClientID, Scopes: row.Scopes,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revoked: row.RevokedAt.Valid,
	}
}
