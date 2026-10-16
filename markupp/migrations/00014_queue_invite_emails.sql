-- +goose Up
-- Fila do email de convite (ADR-0021). A API grava a notificação na mesma transação do
-- convite, e o serviço de notificação entrega depois, gerando o token na entrega: o
-- token nunca fica legível no banco, só o hash dele.
CREATE TABLE notifications (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id       TEXT NOT NULL DEFAULT current_setting('markupp.tenant_id', true),
    invite_id       TEXT NOT NULL REFERENCES user_invites (id) ON DELETE CASCADE,
    attempts        INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_due_idx ON notifications (next_attempt_at)
    WHERE sent_at IS NULL AND failed_at IS NULL;
-- O cascade da revogação acha a notificação do convite pelo índice.
CREATE INDEX notifications_invite_idx ON notifications (invite_id);

-- Papel do serviço de notificação: lê e marca a fila de todos os tenants, e só toca no
-- convite pela função que troca o hash.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE ROLE markupp_notifier NOLOGIN NOSUPERUSER NOBYPASSRLS;
EXCEPTION WHEN duplicate_object OR unique_violation THEN
    NULL;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user) THEN
        EXECUTE format('GRANT markupp_notifier TO %I', current_user);
    END IF;
END
$$;
-- +goose StatementEnd

-- Troca o hash do convite ainda pendente e devolve o que o email mostra. Convite
-- respondido, revogado, vencido ou sem quem convidou no papel não devolve linha: é o
-- mesmo critério do aceite, que recusaria o token. Convite travado por outra
-- transação está sendo aceito, recusado ou revogado: é pulado, senão a revogação, que
-- trava o convite e depois a notificação, e a entrega, que trava na ordem inversa,
-- entram em deadlock.
-- +goose StatementBegin
CREATE FUNCTION markupp_reissue_invite_token(p_invite_id TEXT, p_token_hash TEXT)
RETURNS TABLE (invite_email TEXT, invite_role TEXT, tenant_name TEXT, inviter_email TEXT, invite_expires_at TIMESTAMPTZ)
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    UPDATE user_invites i SET token_hash = p_token_hash
    FROM tenants t, users u
    WHERE i.id = (SELECT id FROM user_invites WHERE id = p_invite_id FOR UPDATE SKIP LOCKED)
      AND i.accepted_at IS NULL AND i.declined_at IS NULL AND i.expires_at > now()
      AND t.id = i.tenant_id AND u.id = i.invited_by
      AND EXISTS (
          SELECT 1 FROM tenant_memberships quem
          WHERE quem.tenant_id = i.tenant_id AND quem.user_id = i.invited_by
            AND markupp_role_rank(quem.role) >= markupp_role_rank(i.role)
      )
    RETURNING i.email, i.role, t.name, u.email, i.expires_at
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_reissue_invite_token(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_reissue_invite_token(TEXT, TEXT) TO markupp_notifier;

-- Consulta do scaler do KEDA: notificações vencidas e ainda abertas.
-- +goose StatementBegin
CREATE FUNCTION markupp_pending_notifications() RETURNS BIGINT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT count(*) FROM notifications
    WHERE sent_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO markupp_notifier;
GRANT SELECT, UPDATE ON notifications TO markupp_notifier;
GRANT INSERT ON notifications TO markupp_app;

ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
CREATE POLICY notifications_enqueue ON notifications FOR INSERT TO markupp_app
    WITH CHECK (tenant_id = current_setting('markupp.tenant_id', true));
CREATE POLICY notifications_notifier ON notifications TO markupp_notifier
    USING (true);

-- +goose Down
DROP POLICY IF EXISTS notifications_notifier ON notifications;
DROP POLICY IF EXISTS notifications_enqueue ON notifications;
REVOKE ALL ON notifications FROM markupp_app, markupp_notifier;
REVOKE USAGE ON SCHEMA public FROM markupp_notifier;
DROP FUNCTION IF EXISTS markupp_pending_notifications();
DROP FUNCTION IF EXISTS markupp_reissue_invite_token(TEXT, TEXT);
DROP TABLE IF EXISTS notifications;
