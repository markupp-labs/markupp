-- +goose Up
-- Verificação de email e troca de senha por email (ADR-0021). O pedido nasce sem token, e
-- o serviço de notificação grava o hash do token e a validade na entrega, como no convite.
CREATE TABLE email_verifications (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE,
    expires_at TIMESTAMPTZ,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX email_verifications_user_idx ON email_verifications (user_id);

CREATE TABLE password_resets (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE,
    expires_at TIMESTAMPTZ,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX password_resets_user_idx ON password_resets (user_id);

-- A notificação aponta para exatamente um pedido. A da conta não tem tenant: a conta é
-- da instância (ADR-0035).
ALTER TABLE notifications
    ALTER COLUMN tenant_id DROP NOT NULL,
    ALTER COLUMN invite_id DROP NOT NULL,
    ADD COLUMN email_verification_id TEXT REFERENCES email_verifications (id) ON DELETE CASCADE,
    ADD COLUMN password_reset_id TEXT REFERENCES password_resets (id) ON DELETE CASCADE,
    ADD CONSTRAINT notifications_one_target
        CHECK (num_nonnulls(invite_id, email_verification_id, password_reset_id) = 1);

-- Grava o hash e a validade do pedido de verificação ainda aberto, de conta ainda sem
-- email verificado, e devolve o email dela.
-- +goose StatementBegin
CREATE FUNCTION markupp_reissue_email_verification(p_id TEXT, p_token_hash TEXT, p_ttl INTERVAL)
RETURNS TABLE (account_email TEXT, token_expires_at TIMESTAMPTZ)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    UPDATE email_verifications v SET token_hash = p_token_hash, expires_at = now() + p_ttl
    FROM users u
    WHERE v.id = p_id AND v.used_at IS NULL AND u.id = v.user_id AND NOT u.email_verified
    RETURNING u.email, v.expires_at
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION markupp_reissue_password_reset(p_id TEXT, p_token_hash TEXT, p_ttl INTERVAL)
RETURNS TABLE (account_email TEXT, token_expires_at TIMESTAMPTZ)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    UPDATE password_resets r SET token_hash = p_token_hash, expires_at = now() + p_ttl
    FROM users u
    WHERE r.id = p_id AND r.used_at IS NULL AND u.id = r.user_id
    RETURNING u.email, r.expires_at
$$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION markupp_reissue_email_verification(TEXT, TEXT, INTERVAL) FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_reissue_password_reset(TEXT, TEXT, INTERVAL) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_reissue_email_verification(TEXT, TEXT, INTERVAL) TO markupp_notifier;
GRANT EXECUTE ON FUNCTION markupp_reissue_password_reset(TEXT, TEXT, INTERVAL) TO markupp_notifier;

-- +goose Down
DROP FUNCTION IF EXISTS markupp_reissue_password_reset(TEXT, TEXT, INTERVAL);
DROP FUNCTION IF EXISTS markupp_reissue_email_verification(TEXT, TEXT, INTERVAL);
DELETE FROM notifications WHERE invite_id IS NULL;
ALTER TABLE notifications
    DROP CONSTRAINT notifications_one_target,
    DROP COLUMN password_reset_id,
    DROP COLUMN email_verification_id,
    ALTER COLUMN invite_id SET NOT NULL,
    ALTER COLUMN tenant_id SET NOT NULL;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS email_verifications;
