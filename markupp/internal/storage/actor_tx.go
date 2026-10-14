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

// enterActorSQL troca para o papel sem privilégio e entrega ao RLS o tenant e o
// usuário. O terceiro argumento true limita os três à transação, como SET LOCAL.
const enterActorSQL = `SELECT set_config('role', 'markupp_app', true),
	set_config('markupp.tenant_id', $1, true),
	set_config('markupp.user_id', $2, true)`

// inActorTx roda fn numa transação em que o banco só enxerga o tenant e os cofres
// do ator do contexto.
func inActorTx(ctx context.Context, pool *pgxpool.Pool, fn func(q *gen.Queries) error) error {
	actor, ok := identity.ActorFrom(ctx)
	if !ok {
		return ErrNoActor
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("iniciar transacao do usuario %q: %w", actor.UserID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, enterActorSQL, actor.TenantID, actor.UserID); err != nil {
		return fmt.Errorf("entrar no tenant %q como %q: %w", actor.TenantID, actor.UserID, err)
	}
	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
