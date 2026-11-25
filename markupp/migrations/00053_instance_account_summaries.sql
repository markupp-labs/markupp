-- +goose Up
-- A conta como o administrador da instância a vê (ADR-0061): os jeitos de entrar e o
-- último acesso, que é o último uso de sessão ou de app autorizado. Sem acesso,
-- has_last_access é falso e last_access_at repete a criação, porque o tipo não é nulo no Go.
CREATE VIEW instance_account_summaries AS
SELECT u.id, u.email, u.email_verified, u.created_at,
       (u.blocked_at IS NOT NULL)::BOOLEAN AS blocked,
       COALESCE((SELECT array_agg(DISTINCT i.provider ORDER BY i.provider)
                 FROM user_identities i WHERE i.user_id = u.id), '{}')::TEXT[] AS login_methods,
       (last.at IS NOT NULL)::BOOLEAN AS has_last_access,
       COALESCE(last.at, u.created_at)::TIMESTAMPTZ AS last_access_at
FROM users u
CROSS JOIN LATERAL (
    SELECT GREATEST(
        (SELECT max(s.last_used_at) FROM user_sessions s WHERE s.user_id = u.id),
        (SELECT max(a.last_used_at) FROM oauth_authorizations a WHERE a.user_id = u.id)) AS at
) last;

REVOKE ALL ON instance_account_summaries
    FROM PUBLIC, markupp_app, markupp_scaler, markupp_indexer, markupp_notifier, markupp_reader;

-- +goose Down
DROP VIEW IF EXISTS instance_account_summaries;
