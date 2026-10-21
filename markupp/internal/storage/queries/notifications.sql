-- A API só enfileira, no tenant do ator. O serviço de notificação trabalha no papel
-- markupp_notifier, que enxerga a fila de todos os tenants e nenhum convite nem token
-- da conta: só os troca pelas funções do banco.

-- name: EnqueueInviteNotification :exec
INSERT INTO notifications (invite_id) VALUES ($1);

-- name: EnterNotifier :exec
SELECT set_config('role', 'markupp_notifier', true);

-- Reserva a próxima notificação vencida por lease_seconds e conta a tentativa já
-- aqui: o envio acontece fora de transação, e um notificador que cai no meio dele
-- também gasta uma.
-- name: ClaimDueNotification :one
UPDATE notifications
SET attempts = attempts + 1, next_attempt_at = now() + sqlc.arg(lease_seconds)::int * interval '1 second'
WHERE id = (
    SELECT id FROM notifications
    WHERE sent_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
    ORDER BY next_attempt_at, id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, attempts, kind, COALESCE(invite_id, account_token_id)::text AS target_id;

-- name: ReissueInviteToken :many
SELECT invite_email::text, invite_role::text, tenant_name::text, inviter_email::text,
       invite_expires_at::timestamptz
FROM markupp_reissue_invite_token(sqlc.arg(invite_id)::text, sqlc.arg(token_hash)::text);

-- name: ReissueAccountToken :many
SELECT account_email::text, token_expires_at::timestamptz
FROM markupp_reissue_account_token(sqlc.arg(id)::text, sqlc.arg(purpose)::text,
                                   sqlc.arg(token_hash)::text, sqlc.arg(ttl)::interval);

-- name: MarkNotificationSent :exec
UPDATE notifications SET sent_at = now() WHERE id = $1;

-- name: MarkNotificationRetry :exec
UPDATE notifications
SET next_attempt_at = sqlc.arg(next_attempt_at), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);

-- name: MarkNotificationFailed :exec
UPDATE notifications
SET failed_at = now(), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);
