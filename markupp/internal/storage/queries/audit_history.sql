-- name: ListAuditEvents :many
-- Os filtros vazios valem tudo; o período é [since, until).
SELECT e.id, COALESCE(e.actor_id, '')::TEXT AS actor_id, COALESCE(u.email, '')::TEXT AS actor_email,
       e.action, e.target, e.result, e.client_ip, e.user_agent, e.occurred_at
FROM audit_events e
LEFT JOIN users u ON u.id = e.actor_id
WHERE (sqlc.arg(account)::TEXT = '' OR e.actor_id = sqlc.arg(account)::TEXT)
  AND (sqlc.arg(action)::TEXT = '' OR e.action = sqlc.arg(action)::TEXT)
  AND (sqlc.arg(result)::TEXT = '' OR e.result = sqlc.arg(result)::TEXT)
  AND (sqlc.arg(ip)::TEXT = '' OR e.client_ip = sqlc.arg(ip)::TEXT)
  AND (NOT sqlc.arg(has_since)::BOOLEAN OR e.occurred_at >= sqlc.arg(since)::TIMESTAMPTZ)
  AND (NOT sqlc.arg(has_until)::BOOLEAN OR e.occurred_at < sqlc.arg(until)::TIMESTAMPTZ)
ORDER BY e.occurred_at DESC, e.id DESC
OFFSET sqlc.arg(row_offset)::INT LIMIT sqlc.arg(row_limit)::INT;
