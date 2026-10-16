-- A API só enfileira, no tenant do ator. O serviço de notificação trabalha no papel
-- markupp_notifier, que enxerga a fila de todos os tenants e nenhum convite.

-- name: EnqueueInviteNotification :exec
INSERT INTO notifications (invite_id) VALUES ($1);

-- name: EnterNotifier :exec
SELECT set_config('role', 'markupp_notifier', true);

-- name: ReserveDueNotification :one
SELECT id, attempts,
       (CASE WHEN invite_id IS NOT NULL THEN 'invite'
             WHEN email_verification_id IS NOT NULL THEN 'email_verification'
             ELSE 'password_reset' END)::text AS kind,
       COALESCE(invite_id, email_verification_id, password_reset_id)::text AS target_id
FROM notifications
WHERE sent_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
ORDER BY next_attempt_at, id
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: ReissueInviteToken :many
SELECT invite_email::text, invite_role::text, tenant_name::text, inviter_email::text,
       invite_expires_at::timestamptz
FROM markupp_reissue_invite_token(sqlc.arg(invite_id)::text, sqlc.arg(token_hash)::text);

-- name: MarkNotificationSent :exec
UPDATE notifications SET sent_at = now() WHERE id = $1;

-- name: MarkNotificationRetry :exec
UPDATE notifications
SET attempts = attempts + 1, next_attempt_at = sqlc.arg(next_attempt_at), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);

-- name: MarkNotificationFailed :exec
UPDATE notifications
SET attempts = attempts + 1, failed_at = now(), last_error = sqlc.arg(last_error)
WHERE id = sqlc.arg(id);

-- name: ReissueEmailVerification :many
SELECT account_email::text, token_expires_at::timestamptz
FROM markupp_reissue_email_verification(sqlc.arg(id)::text, sqlc.arg(token_hash)::text, sqlc.arg(ttl)::interval);

-- name: ReissuePasswordReset :many
SELECT account_email::text, token_expires_at::timestamptz
FROM markupp_reissue_password_reset(sqlc.arg(id)::text, sqlc.arg(token_hash)::text, sqlc.arg(ttl)::interval);
