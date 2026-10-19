package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/oauth"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// ConsumeRefreshToken rotaciona o refresh token e lê a Autorização ativa na mesma
// transação. Sem Autorização ativa, a rotação fica gravada e o erro volta junto com o
// grant. Um refresh já rotacionado volta com oauth.ErrRefreshReused.
func (r *PostgresOAuthRepository) ConsumeRefreshToken(ctx context.Context, tokenHash string, now time.Time) (oauth.TokenGrant, error) {
	var grant oauth.TokenGrant
	var missing error
	err := inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		var err error
		grant, err = consumeRefreshInTx(ctx, qtx, tokenHash, now)
		if errors.Is(err, oauth.ErrAuthorizationNotFound) {
			missing, err = err, nil
		}
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return r.rotatedRefresh(ctx, tokenHash)
	}
	if err != nil {
		return oauth.TokenGrant{}, err
	}
	return grant, missing
}

// consumeRefreshInTx marca o refresh como rotacionado e busca a Autorização ativa dele.
func consumeRefreshInTx(ctx context.Context, qtx *gen.Queries, tokenHash string, now time.Time) (oauth.TokenGrant, error) {
	row, err := qtx.ConsumeOAuthRefreshToken(ctx, gen.ConsumeOAuthRefreshTokenParams{TokenHash: tokenHash, Now: now})
	if err != nil {
		return oauth.TokenGrant{}, fmt.Errorf("consumir refresh token oauth: %w", err)
	}
	token := toIssuedToken(row)
	authz, err := activeAuthorizationByID(ctx, qtx, token.AuthorizationID)
	return oauth.TokenGrant{Token: token, Authorization: authz}, err
}

// rotatedRefresh distingue o reuso, refresh rotacionado e ainda não revogado, do refresh
// inexistente, expirado ou revogado. O reuso volta com a Autorização dona, mesmo
// revogada, para a auditoria ter ator e alvo.
func (r *PostgresOAuthRepository) rotatedRefresh(ctx context.Context, tokenHash string) (oauth.TokenGrant, error) {
	row, err := r.q.GetOAuthTokenWithAuthorization(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.TokenGrant{}, oauth.ErrTokenNotFound
	}
	if err != nil {
		return oauth.TokenGrant{}, fmt.Errorf("buscar refresh token oauth já rotacionado: %w", err)
	}
	token := row.OauthToken
	if token.Kind != oauth.TokenKindRefresh || !token.RotatedAt.Valid || token.Revoked {
		return oauth.TokenGrant{}, oauth.ErrTokenNotFound
	}
	return oauth.TokenGrant{Token: toIssuedToken(token), Authorization: toAuthorization(row.OauthAuthorization)}, oauth.ErrRefreshReused
}

// activeAuthorizationByID busca a Autorização não revogada pelo ID, por q.
func activeAuthorizationByID(ctx context.Context, q *gen.Queries, id string) (oauth.Authorization, error) {
	row, err := q.GetActiveOAuthAuthorizationByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.Authorization{}, fmt.Errorf("%w: autorização %q", oauth.ErrAuthorizationNotFound, id)
	}
	if err != nil {
		return oauth.Authorization{}, fmt.Errorf("buscar autorização %q: %w", id, err)
	}
	return toAuthorization(row), nil
}

// SaveRefreshedTokens trava a Autorização do par novo e, se ela segue ativa e o refresh
// pai não foi revogado, grava o par e audita o evento, numa transação. A trava serializa
// a gravação com as revogações, que travam a mesma linha.
func (r *PostgresOAuthRepository) SaveRefreshedTokens(ctx context.Context, parentHash string, tokens []oauth.IssuedToken, event oauth.AuditEvent) error {
	if len(tokens) == 0 {
		return errors.New("gravar par rotacionado: nenhum token recebido, esperado o access e o refresh novos")
	}
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		if err := checkRefreshParentInTx(ctx, qtx, parentHash, tokens[0].AuthorizationID); err != nil {
			return err
		}
		if err := saveTokensInTx(ctx, qtx, tokens); err != nil {
			return err
		}
		return auditOAuth(ctx, qtx, event)
	})
}

// checkRefreshParentInTx trava a Autorização e confere que ela segue ativa e que o
// refresh pai dela não foi revogado.
func checkRefreshParentInTx(ctx context.Context, qtx *gen.Queries, parentHash, authorizationID string) error {
	row, err := lockAuthorization(ctx, qtx, authorizationID)
	if err != nil {
		return err
	}
	if row.RevokedAt.Valid {
		return fmt.Errorf("%w: autorização %q revogada antes de gravar o par rotacionado", oauth.ErrAuthorizationNotFound, authorizationID)
	}
	live, err := qtx.IsUnrevokedOAuthToken(ctx, gen.IsUnrevokedOAuthTokenParams{TokenHash: parentHash, AuthorizationID: authorizationID})
	if err != nil {
		return fmt.Errorf("conferir refresh pai da autorização %q: %w", authorizationID, err)
	}
	if !live {
		return fmt.Errorf("%w: refresh pai da autorização %q revogado antes de gravar o par rotacionado", oauth.ErrTokenNotFound, authorizationID)
	}
	return nil
}

// lockAuthorization trava a linha da Autorização, revogada ou não, até o fim da
// transação, ou devolve oauth.ErrAuthorizationNotFound se ela não existe.
func lockAuthorization(ctx context.Context, qtx *gen.Queries, id string) (gen.OauthAuthorization, error) {
	row, err := qtx.LockOAuthAuthorization(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, fmt.Errorf("%w: autorização %s", oauth.ErrAuthorizationNotFound, oauth.ClientValue(id))
	}
	if err != nil {
		return row, fmt.Errorf("travar autorização %s: %w", oauth.ClientValue(id), err)
	}
	return row, nil
}

// RevokeAuthorizationTokensAudited trava a Autorização, revoga os tokens dela, que segue
// ativa, e audita o evento, numa transação.
func (r *PostgresOAuthRepository) RevokeAuthorizationTokensAudited(ctx context.Context, authorizationID string, event oauth.AuditEvent) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		if _, err := revokeTokens(ctx, qtx, authorizationID); err != nil {
			return err
		}
		return auditOAuth(ctx, qtx, event)
	})
}

// FindToken devolve o token de qualquer tipo, mesmo revogado ou expirado, com a
// Autorização dona, mesmo revogada, ou oauth.ErrTokenNotFound.
func (r *PostgresOAuthRepository) FindToken(ctx context.Context, tokenHash string) (oauth.TokenGrant, error) {
	row, err := r.q.GetOAuthTokenWithAuthorization(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.TokenGrant{}, oauth.ErrTokenNotFound
	}
	if err != nil {
		return oauth.TokenGrant{}, fmt.Errorf("buscar token oauth: %w", err)
	}
	return oauth.TokenGrant{Token: toIssuedToken(row.OauthToken), Authorization: toAuthorization(row.OauthAuthorization)}, nil
}

// ListActiveAuthorizations devolve as Autorizações ativas da conta com o nome do cliente,
// da atualizada mais recentemente para a mais antiga.
func (r *PostgresOAuthRepository) ListActiveAuthorizations(ctx context.Context, userID string) ([]oauth.AuthorizationSummary, error) {
	rows, err := r.q.ListActiveOAuthAuthorizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listar autorizações ativas de %q: %w", userID, err)
	}
	summaries := make([]oauth.AuthorizationSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, oauth.AuthorizationSummary{
			Authorization: toAuthorization(row.OauthAuthorization), ClientName: row.ClientName,
		})
	}
	return summaries, nil
}

// RevokeAuthorization trava a Autorização e revoga a ativa da conta em
// event.OccurredAt, os tokens dela, e audita o evento com o cliente dela como alvo,
// numa transação.
func (r *PostgresOAuthRepository) RevokeAuthorization(ctx context.Context, userID, authorizationID string, event oauth.AuditEvent) error {
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		row, err := revokeTokens(ctx, qtx, authorizationID)
		if err != nil {
			return err
		}
		event.ClientID = row.ClientID
		if err := revokeAuthorizationInTx(ctx, qtx, userID, authorizationID, event.OccurredAt); err != nil {
			return err
		}
		return auditOAuth(ctx, qtx, event)
	})
}

// revokeAuthorizationInTx marca a Autorização como revogada, ou devolve
// oauth.ErrAuthorizationNotFound se ela não existe, já foi revogada ou é de outra conta.
func revokeAuthorizationInTx(ctx context.Context, qtx *gen.Queries, userID, authorizationID string, at time.Time) error {
	n, err := qtx.RevokeOAuthAuthorization(ctx, gen.RevokeOAuthAuthorizationParams{ID: authorizationID, UserID: userID, At: at})
	if err != nil {
		return fmt.Errorf("revogar autorização %s de %q: %w", oauth.ClientValue(authorizationID), userID, err)
	}
	if n == 0 {
		return fmt.Errorf("%w: autorização %s da conta %q", oauth.ErrAuthorizationNotFound, oauth.ClientValue(authorizationID), userID)
	}
	return nil
}

func toIssuedToken(row gen.OauthToken) oauth.IssuedToken {
	return oauth.IssuedToken{
		Hash: row.TokenHash, AuthorizationID: row.AuthorizationID, Kind: row.Kind,
		Resource: row.Resource, Scopes: row.Scopes, ExpiresAt: row.ExpiresAt, Revoked: row.Revoked,
	}
}
