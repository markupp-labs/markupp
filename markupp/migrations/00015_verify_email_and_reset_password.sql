-- +goose Up
-- Verificação de email e troca de senha por email (ADR-0021). O pedido nasce sem token, e
-- o serviço de notificação grava o hash do token e a validade na entrega, como no convite.
-- Validade e intervalo entre pedidos seguem o relógio do banco.
CREATE TABLE account_tokens (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    TEXT NOT NULL CHECK (purpose IN ('email_verification', 'password_reset')),
    token_hash TEXT UNIQUE,
    expires_at TIMESTAMPTZ,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX account_tokens_user_purpose_idx ON account_tokens (user_id, purpose);

-- Cada notificação aponta para o alvo do seu tipo. A de convite carrega o tenant dele; a
-- da conta não tem tenant, porque a conta é da instância (ADR-0035).
ALTER TABLE notifications
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'invite'
        CHECK (kind IN ('invite', 'email_verification', 'password_reset')),
    ALTER COLUMN tenant_id DROP NOT NULL,
    ALTER COLUMN invite_id DROP NOT NULL,
    ADD COLUMN account_token_id TEXT REFERENCES account_tokens (id) ON DELETE CASCADE,
    ADD CONSTRAINT notifications_target_matches_kind CHECK (
        (kind = 'invite' AND invite_id IS NOT NULL AND account_token_id IS NULL AND tenant_id IS NOT NULL)
        OR (kind <> 'invite' AND invite_id IS NULL AND account_token_id IS NOT NULL)
    );
CREATE INDEX notifications_account_token_idx ON notifications (account_token_id);

-- Grava o hash e a validade do token ainda aberto e devolve o email da conta. Token de
-- verificação de conta já verificada não volta. Token travado por outra transação está
-- sendo trocado por um reenvio: é pulado, pelo mesmo motivo do convite (00014).
-- +goose StatementBegin
CREATE FUNCTION markupp_reissue_account_token(p_id TEXT, p_purpose TEXT, p_token_hash TEXT, p_ttl INTERVAL)
RETURNS TABLE (account_email TEXT, token_expires_at TIMESTAMPTZ)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    UPDATE account_tokens a SET token_hash = p_token_hash, expires_at = now() + p_ttl
    FROM users u
    WHERE a.id = (SELECT id FROM account_tokens WHERE id = p_id AND purpose = p_purpose FOR UPDATE SKIP LOCKED)
      AND a.used_at IS NULL AND u.id = a.user_id
      AND (a.purpose <> 'email_verification' OR NOT u.email_verified)
    RETURNING u.email, a.expires_at
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_reissue_account_token(TEXT, TEXT, TEXT, INTERVAL) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_reissue_account_token(TEXT, TEXT, TEXT, INTERVAL) TO markupp_notifier;

-- +goose Down
DROP FUNCTION IF EXISTS markupp_reissue_account_token(TEXT, TEXT, TEXT, INTERVAL);
DELETE FROM notifications WHERE kind <> 'invite';
ALTER TABLE notifications
    DROP CONSTRAINT notifications_target_matches_kind,
    DROP COLUMN account_token_id,
    DROP COLUMN kind,
    ALTER COLUMN invite_id SET NOT NULL,
    ALTER COLUMN tenant_id SET NOT NULL;
DROP TABLE IF EXISTS account_tokens;
