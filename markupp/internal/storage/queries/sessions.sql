-- name: ListActiveUserSessions :many
-- Ativa é a sessão sem fim com algum refresh que ainda renova.
SELECT s.id, s.user_id, s.client_ip, s.user_agent, s.created_at, s.last_used_at
FROM user_sessions s
WHERE s.user_id = $1 AND s.ended_at IS NULL
  AND EXISTS (SELECT 1 FROM refresh_tokens r
              WHERE r.session_id = s.id AND r.revoked = FALSE AND r.expires_at > now())
ORDER BY s.last_used_at DESC, s.id;

-- name: EndUserSession :one
-- Encerra a sessão da conta e revoga os refresh dela; conta as sessões encerradas.
WITH ended AS (
    UPDATE user_sessions s SET ended_at = now()
    WHERE s.id = $1 AND s.user_id = $2 AND s.ended_at IS NULL
    RETURNING s.id
), revoked AS (
    UPDATE refresh_tokens r SET revoked = TRUE
    WHERE r.session_id IN (SELECT id FROM ended) AND r.revoked = FALSE
)
SELECT count(*) FROM ended;
