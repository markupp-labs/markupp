-- name: GetInstanceRetentionDays :one
SELECT COALESCE(days, 0)::INT FROM instance_retention;

-- name: SetInstanceRetentionDays :exec
UPDATE instance_retention SET days = $1, updated_at = clock_timestamp();

-- name: TryRetentionCleanupLock :one
-- Uma réplica por vez; a trava some com a transação.
SELECT pg_try_advisory_xact_lock(hashtext('markupp.retention_cleanup'));

-- name: MarkRetentionCleanupIfDue :execrows
UPDATE instance_retention SET last_cleanup_at = sqlc.arg(now)::TIMESTAMPTZ
WHERE last_cleanup_at IS NULL OR last_cleanup_at < sqlc.arg(due_after)::TIMESTAMPTZ;

-- name: EnableAuditRetention :exec
SELECT set_config('markupp.audit_retention', 'on', true);

-- name: DeleteAuditEventsBefore :execrows
DELETE FROM audit_events WHERE occurred_at < $1;

-- name: ClearEndedSessionOrigins :execrows
-- A sessão ativa guarda a origem; a encerrada há mais que o prazo a perde.
UPDATE user_sessions SET client_ip = '', user_agent = ''
WHERE ended_at < $1 AND (client_ip <> '' OR user_agent <> '');
