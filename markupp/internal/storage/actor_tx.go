package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/identity"
	"github.com/ifsc-ES2/projeto-markupp/markupp/internal/storage/gen"
)

// ErrNoActor indica transação de dados pedida sem ator autenticado no contexto.
var ErrNoActor = errors.New("transacao de dados sem ator autenticado no contexto")

// inTx roda fn numa transação, desfeita se fn falhar.
func inTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// inActorTx roda fn numa transação em que o banco só enxerga o tenant e os cofres
// do ator do contexto, e as políticas de administração conferem o papel dele.
func inActorTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error {
	actor, ok := identity.ActorFrom(ctx)
	if !ok {
		return ErrNoActor
	}
	return inTx(ctx, pool, func(q *gen.Queries) error {
		if err := q.EnterActor(ctx, gen.EnterActorParams{TenantID: actor.TenantID, UserID: actor.UserID, Role: actor.Role}); err != nil {
			return fmt.Errorf("entrar no tenant %q como %q: %w", actor.TenantID, actor.UserID, err)
		}
		return fn(q)
	})
}

// inPublicTx roda fn numa transação no papel markupp_reader (ADR-0048), que só lê, e
// só as notas e o plano de recuperação dos cofres publicados. É a única entrada de
// dados sem ator, e não carrega nada do contexto para a sessão.
func inPublicTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error {
	return inTx(ctx, pool, func(q *gen.Queries) error {
		if err := q.EnterPublicReader(ctx); err != nil {
			return fmt.Errorf("entrar no papel da leitura publica: %w", err)
		}
		return fn(q)
	})
}
