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
// authorization server OAuth. As tabelas são da instância (ADR-0038): sem ator nem RLS.
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
		return oauth.Client{}, fmt.Errorf("%w: client_id %q", oauth.ErrClientNotFound, id)
	}
	if err != nil {
		return oauth.Client{}, fmt.Errorf("buscar cliente oauth %q: %w", id, err)
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
		return fmt.Errorf("gravar pedido de autorização do cliente %q: %w", req.ClientID, err)
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
func (r *PostgresOAuthRepository) ApproveRequest(ctx context.Context, a oauth.Approval) (oauth.AuthorizationRequest, error) {
	var row gen.OauthRequest
	err := inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		var err error
		row, err = qtx.ApproveOAuthRequest(ctx, gen.ApproveOAuthRequestParams{
			ID: a.RequestID, UserID: a.UserID, CodeHash: a.CodeHash,
			CodeExpiresAt: a.CodeExpiresAt, DecidedAt: a.DecidedAt,
		})
		if err != nil {
			return requestErr("aprovar pedido", err)
		}
		if err := grantScopes(ctx, qtx, row, a.DecidedAt); err != nil {
			return err
		}
		return qtx.CreateAuditEvent(ctx, authorizeEvent(row, "success", a.DecidedAt))
	})
	if err != nil {
		return oauth.AuthorizationRequest{}, err
	}
	return toAuthorizationRequest(row), nil
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
		return qtx.CreateAuditEvent(ctx, authorizeEvent(row, "failure", d.DecidedAt))
	})
	if err != nil {
		return oauth.AuthorizationRequest{}, err
	}
	return toAuthorizationRequest(row), nil
}

// grantScopes amplia a Autorização ativa da conta para o cliente com os escopos do
// pedido, ou a cria. A trava na linha serializa aprovações concorrentes do mesmo par.
func grantScopes(ctx context.Context, qtx *gen.Queries, req gen.OauthRequest, at time.Time) error {
	userID := req.UserID.String
	current, err := qtx.LockActiveOAuthAuthorization(ctx, gen.LockActiveOAuthAuthorizationParams{UserID: userID, ClientID: req.ClientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return createAuthorization(ctx, qtx, req, at)
	}
	if err != nil {
		return fmt.Errorf("travar autorização de %q para %q: %w", userID, req.ClientID, err)
	}
	err = qtx.UpdateOAuthAuthorizationScopes(ctx, gen.UpdateOAuthAuthorizationScopesParams{
		ID: current.ID, Scopes: unionScopes(current.Scopes, req.Scopes), UpdatedAt: at,
	})
	if err != nil {
		return fmt.Errorf("ampliar escopos da autorização %q: %w", current.ID, err)
	}
	return nil
}

func createAuthorization(ctx context.Context, qtx *gen.Queries, req gen.OauthRequest, at time.Time) error {
	err := qtx.CreateOAuthAuthorization(ctx, gen.CreateOAuthAuthorizationParams{
		ID: uuid.NewString(), UserID: req.UserID.String, ClientID: req.ClientID,
		Scopes: unionScopes(nil, req.Scopes), CreatedAt: at,
	})
	if err != nil {
		return fmt.Errorf("criar autorização de %q para %q: %w", req.UserID.String, req.ClientID, err)
	}
	return nil
}

// unionScopes soma os escopos na ordem de oauth.SupportedScopes, sem repetir.
func unionScopes(a, b []string) []string {
	out := make([]string, 0, len(a)+len(b))
	for _, scope := range oauth.SupportedScopes() {
		if slices.Contains(a, scope) || slices.Contains(b, scope) {
			out = append(out, scope)
		}
	}
	return out
}

func authorizeEvent(req gen.OauthRequest, result string, at time.Time) gen.CreateAuditEventParams {
	return toCreateAuditEventParams(auth.AuditEventRecord{
		ID: uuid.NewString(), ActorID: req.UserID.String, Action: auditActionOAuthAuthorize,
		Target: req.ClientID, Result: result, OccurredAt: at,
	})
}

// ActiveAuthorization devolve a Autorização não revogada da conta para o cliente.
func (r *PostgresOAuthRepository) ActiveAuthorization(ctx context.Context, userID, clientID string) (oauth.Authorization, error) {
	row, err := r.q.GetActiveOAuthAuthorization(ctx, gen.GetActiveOAuthAuthorizationParams{UserID: userID, ClientID: clientID})
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Authorization{}, fmt.Errorf("%w: usuario %q, cliente %q", oauth.ErrAuthorizationNotFound, userID, clientID)
	}
	if err != nil {
		return oauth.Authorization{}, fmt.Errorf("buscar autorização de %q para %q: %w", userID, clientID, err)
	}
	return toAuthorization(row), nil
}

// RedeemCode consome o código e devolve o pedido com a Autorização ativa. No reuso,
// devolve o mesmo resultado junto com oauth.ErrCodeAlreadyUsed.
func (r *PostgresOAuthRepository) RedeemCode(ctx context.Context, codeHash string, now time.Time) (oauth.RedeemedCode, error) {
	row, err := r.q.RedeemOAuthCode(ctx, gen.RedeemOAuthCodeParams{CodeHash: codeHash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.usedCode(ctx, codeHash)
	}
	if err != nil {
		return oauth.RedeemedCode{}, fmt.Errorf("consumir código de autorização: %w", err)
	}
	req := toAuthorizationRequest(row)
	authz, err := r.ActiveAuthorization(ctx, req.UserID, req.ClientID)
	return oauth.RedeemedCode{Request: req, Authorization: authz}, err
}

// usedCode distingue o código reusado do inexistente ou expirado.
func (r *PostgresOAuthRepository) usedCode(ctx context.Context, codeHash string) (oauth.RedeemedCode, error) {
	row, err := r.q.GetOAuthRequestByUsedCode(ctx, codeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.RedeemedCode{}, oauth.ErrCodeNotFound
	}
	if err != nil {
		return oauth.RedeemedCode{}, fmt.Errorf("buscar código de autorização já usado: %w", err)
	}
	req := toAuthorizationRequest(row)
	authz, err := r.ActiveAuthorization(ctx, req.UserID, req.ClientID)
	if err != nil && !errors.Is(err, oauth.ErrAuthorizationNotFound) {
		return oauth.RedeemedCode{}, err
	}
	return oauth.RedeemedCode{Request: req, Authorization: authz}, oauth.ErrCodeAlreadyUsed
}

// SaveTokens grava os tokens emitidos, só pelo hash, todos ou nenhum.
func (r *PostgresOAuthRepository) SaveTokens(ctx context.Context, tokens []oauth.IssuedToken) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
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
	})
}

// RevokeAuthorizationTokens revoga todos os tokens da Autorização.
func (r *PostgresOAuthRepository) RevokeAuthorizationTokens(ctx context.Context, authorizationID string) error {
	if err := r.q.RevokeOAuthAuthorizationTokens(ctx, authorizationID); err != nil {
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
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
