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

// RedeemCode consome o código e trava a Autorização gravada no pedido, numa transação.
// Com ela revogada, o consumo fica gravado e oauth.ErrAuthorizationNotFound volta junto
// com o pedido. No reuso, marca o pedido e revoga os tokens da Autorização dele, também
// numa transação, e devolve o pedido com ela, mesmo revogada, e oauth.ErrCodeAlreadyUsed.
func (r *PostgresOAuthRepository) RedeemCode(ctx context.Context, codeHash string, now time.Time, _ time.Duration) (oauth.RedeemedCode, error) {
	var redeemed oauth.RedeemedCode
	var outcome error
	err := inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		var err error
		redeemed, err = redeemInTx(ctx, qtx, codeHash, now)
		if errors.Is(err, oauth.ErrAuthorizationNotFound) || errors.Is(err, oauth.ErrCodeAlreadyUsed) {
			outcome, err = err, nil
		}
		return err
	})
	if err != nil {
		return oauth.RedeemedCode{}, err
	}
	return redeemed, outcome
}

// redeemInTx consome o código ainda não usado ou, se ele já foi, trata o reuso.
func redeemInTx(ctx context.Context, qtx *gen.Queries, codeHash string, now time.Time) (oauth.RedeemedCode, error) {
	row, err := qtx.RedeemOAuthCode(ctx, gen.RedeemOAuthCodeParams{CodeHash: codeHash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return reuseCodeInTx(ctx, qtx, codeHash, now)
	}
	if err != nil {
		return oauth.RedeemedCode{}, fmt.Errorf("consumir código de autorização: %w", err)
	}
	redeemed, err := lockRequestAuthorization(ctx, qtx, row)
	if err == nil && redeemed.Authorization.Revoked {
		err = fmt.Errorf("%w: autorização %q do pedido revogada", oauth.ErrAuthorizationNotFound, redeemed.Authorization.ID)
	}
	return redeemed, err
}

// reuseCodeInTx marca o reuso do código já usado e, com a trava da Autorização do
// pedido, revoga os tokens dela (RFC 9700, 4.4.2) e audita oauth.code_reuse. A marca faz falhar a gravação do par
// do primeiro uso que ainda esteja por vir. Código inexistente ou expirado sem uso é
// oauth.ErrCodeNotFound.
func reuseCodeInTx(ctx context.Context, qtx *gen.Queries, codeHash string, now time.Time) (oauth.RedeemedCode, error) {
	row, err := qtx.MarkOAuthCodeReused(ctx, gen.MarkOAuthCodeReusedParams{CodeHash: codeHash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return oauth.RedeemedCode{}, oauth.ErrCodeNotFound
	}
	if err != nil {
		return oauth.RedeemedCode{}, fmt.Errorf("marcar reuso do código de autorização: %w", err)
	}
	redeemed, err := lockRequestAuthorization(ctx, qtx, row)
	if err != nil {
		return redeemed, err
	}
	if err := revokeLockedTokens(ctx, qtx, redeemed.Authorization.ID); err != nil {
		return redeemed, err
	}
	if err := auditOAuth(ctx, qtx, codeReuseEvent(row, now)); err != nil {
		return redeemed, err
	}
	return redeemed, oauth.ErrCodeAlreadyUsed
}

// codeReuseEvent descreve o reuso do código como falha da conta que o aprovou para o
// cliente do pedido.
func codeReuseEvent(row gen.OauthRequest, at time.Time) oauth.AuditEvent {
	return oauth.AuditEvent{
		Action: oauth.AuditActionCodeReuse, Result: oauth.AuditResultFailure,
		UserID: row.UserID.String, ClientID: row.ClientID, OccurredAt: at,
	}
}

// lockRequestAuthorization trava a Autorização gravada no pedido, revogada ou não, e
// devolve o pedido com ela.
func lockRequestAuthorization(ctx context.Context, qtx *gen.Queries, row gen.OauthRequest) (oauth.RedeemedCode, error) {
	authz, err := lockAuthorization(ctx, qtx, row.AuthorizationID.String)
	if err != nil {
		return oauth.RedeemedCode{}, err
	}
	return oauth.RedeemedCode{Request: toAuthorizationRequest(row), Authorization: toAuthorization(authz)}, nil
}

// SaveCodeTokens trava a Autorização do par e, se ela segue ativa e o código do pedido
// não foi reapresentado, grava os tokens, só pelo hash, todos ou nenhum. A trava
// serializa a gravação com o reuso do código e com as revogações, que travam a mesma
// linha.
func (r *PostgresOAuthRepository) SaveCodeTokens(ctx context.Context, requestID string, tokens []oauth.IssuedToken) error {
	if len(tokens) == 0 {
		return errors.New("gravar par do código: nenhum token recebido, esperado o access e o refresh novos")
	}
	return inTx(ctx, r.pool, func(qtx *gen.Queries) error {
		if err := checkCodeRequestInTx(ctx, qtx, requestID, tokens[0].AuthorizationID); err != nil {
			return err
		}
		return saveTokensInTx(ctx, qtx, tokens)
	})
}

// checkCodeRequestInTx trava a Autorização e confere que ela segue ativa e que o código
// do pedido, que precisa ser dela, não foi reapresentado.
func checkCodeRequestInTx(ctx context.Context, qtx *gen.Queries, requestID, authorizationID string) error {
	if err := lockActiveAuthorization(ctx, qtx, authorizationID, "o par do código"); err != nil {
		return err
	}
	unreused, err := qtx.IsUnreusedOAuthCode(ctx, gen.IsUnreusedOAuthCodeParams{ID: requestID, AuthorizationID: authorizationID})
	if err != nil {
		return fmt.Errorf("conferir o reuso do código da autorização %q: %w", authorizationID, err)
	}
	if !unreused {
		return fmt.Errorf("%w: código da autorização %q reapresentado, ou pedido inexistente ou de outra autorização, esperado o pedido do código ainda sem reuso", oauth.ErrCodeAlreadyUsed, authorizationID)
	}
	return nil
}
