-- +goose Up
-- Limite de convites (#50). A contagem sai da auditoria, que é imutável: revogar o
-- convite apaga a linha de user_invites, e contar lá deixaria convidar, revogar e
-- convidar de novo sem fim.
CREATE INDEX audit_events_invites_by_actor_idx ON audit_events (actor_id, occurred_at)
    WHERE action = 'users.invite';
CREATE INDEX audit_events_invites_by_tenant_idx ON audit_events (tenant_id, occurred_at)
    WHERE action = 'users.invite';

-- Cota de quem convida, lida da sessão do EnterActor. Conta os convites da conta em
-- todos os tenants dela, que o RLS esconderia, e os do tenant. As travas, primeiro a do
-- tenant e depois a da conta, seguram até o fim da transação: dois convites ao mesmo
-- tempo não leem a mesma contagem e estouram o limite juntos.
-- +goose StatementBegin
CREATE FUNCTION markupp_invite_quota(p_since TIMESTAMPTZ)
RETURNS TABLE (inviter_verified BOOLEAN, by_inviter BIGINT, by_tenant BIGINT)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT pg_advisory_xact_lock(hashtextextended('invite-tenant:' || current_setting('markupp.tenant_id', true), 0));
    SELECT pg_advisory_xact_lock(hashtextextended('invite-account:' || current_setting('markupp.user_id', true), 0));
    SELECT u.email_verified,
           (SELECT count(*) FROM audit_events e
            WHERE e.action = 'users.invite' AND e.actor_id = u.id AND e.occurred_at > p_since),
           (SELECT count(*) FROM audit_events e
            WHERE e.action = 'users.invite' AND e.occurred_at > p_since
              AND e.tenant_id = current_setting('markupp.tenant_id', true))
    FROM users u
    WHERE u.id = current_setting('markupp.user_id', true)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_invite_quota(TIMESTAMPTZ) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_invite_quota(TIMESTAMPTZ) TO markupp_app;

-- +goose Down
DROP FUNCTION IF EXISTS markupp_invite_quota(TIMESTAMPTZ);
DROP INDEX IF EXISTS audit_events_invites_by_tenant_idx;
DROP INDEX IF EXISTS audit_events_invites_by_actor_idx;
