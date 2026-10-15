-- +goose Up

-- O usuário passa a ser da instância e participa de N tenants, cada um com um papel,
-- no modelo de organizações do Langfuse (ADR-0035). O cofre segue como unidade de
-- acesso às notas (ADR-0024).
CREATE TABLE tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);

INSERT INTO tenants (id, name, created_at)
SELECT tenant_id, tenant_id, now()
FROM (SELECT tenant_id FROM users UNION SELECT tenant_id FROM vaults) AS existentes;

CREATE TABLE tenant_memberships (
    tenant_id   TEXT NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, user_id),
    CONSTRAINT tenant_memberships_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer'))
);
CREATE INDEX tenant_memberships_user_id_idx ON tenant_memberships (user_id);

-- O admin mais antigo de cada tenant é quem o criou e vira owner.
INSERT INTO tenant_memberships (tenant_id, user_id, role, created_at)
SELECT tenant_id, id,
       CASE
           WHEN role = 'admin' AND row_number() OVER (PARTITION BY tenant_id, role ORDER BY created_at, id) = 1 THEN 'owner'
           ELSE role
       END,
       created_at
FROM users;

ALTER TABLE vaults ADD CONSTRAINT vaults_tenant_fkey
    FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE CASCADE;

-- Convite sobrevive à saída de quem convidou, e aceita os quatro papéis.
ALTER TABLE user_invites
    DROP CONSTRAINT user_invites_invited_by_fkey,
    DROP CONSTRAINT user_invites_role_check,
    ALTER COLUMN invited_by DROP NOT NULL,
    ADD CONSTRAINT user_invites_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES users (id) ON DELETE SET NULL,
    ADD CONSTRAINT user_invites_tenant_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE CASCADE,
    ADD CONSTRAINT user_invites_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer'));

DROP POLICY users_isolation ON users;
ALTER TABLE users
    DROP CONSTRAINT users_tenant_id_key,
    DROP CONSTRAINT users_tenant_email_key,
    DROP CONSTRAINT users_role_check,
    DROP COLUMN tenant_id,
    DROP COLUMN role,
    ADD CONSTRAINT users_email_key UNIQUE (email);

-- Quem é do tenant vê os colegas, e só eles.
CREATE POLICY users_isolation ON users
    USING (id IN (SELECT user_id FROM tenant_memberships
                  WHERE tenant_id = current_setting('markupp.tenant_id', true)));

-- Login, refresh e cadastro são eventos da conta, sem tenant.
ALTER TABLE audit_events ALTER COLUMN tenant_id DROP NOT NULL;

DROP POLICY user_invites_isolation ON user_invites;
CREATE POLICY user_invites_isolation ON user_invites
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) IN ('owner', 'admin'));

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenants_isolation ON tenants
    USING (id = current_setting('markupp.tenant_id', true));

ALTER TABLE tenant_memberships ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_memberships_isolation ON tenant_memberships
    USING (tenant_id = current_setting('markupp.tenant_id', true));

GRANT SELECT ON tenants, tenant_memberships TO markupp_app;

-- +goose Down
-- Destrutivo: cada usuário volta a um tenant só, o do vínculo mais antigo; conta sem
-- vínculo, convite sem quem convidou e evento de conta sem tenant são apagados.
REVOKE ALL ON tenants, tenant_memberships FROM markupp_app;
DROP POLICY user_invites_isolation ON user_invites;
CREATE POLICY user_invites_isolation ON user_invites
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) = 'admin');
ALTER TABLE audit_events DISABLE TRIGGER audit_events_prevent_mutation;
DELETE FROM audit_events WHERE tenant_id IS NULL;
ALTER TABLE audit_events ENABLE TRIGGER audit_events_prevent_mutation;
ALTER TABLE audit_events ALTER COLUMN tenant_id SET NOT NULL;
DROP POLICY users_isolation ON users;
ALTER TABLE users DROP CONSTRAINT users_email_key, ADD COLUMN tenant_id TEXT, ADD COLUMN role TEXT;
UPDATE users u SET tenant_id = m.tenant_id,
                   role = CASE WHEN m.role IN ('owner', 'admin') THEN 'admin' ELSE 'member' END
FROM (SELECT DISTINCT ON (user_id) user_id, tenant_id, role FROM tenant_memberships
      ORDER BY user_id, created_at) AS m
WHERE m.user_id = u.id;
DELETE FROM users WHERE tenant_id IS NULL;
ALTER TABLE users
    ALTER COLUMN tenant_id SET NOT NULL,
    ALTER COLUMN role SET NOT NULL,
    ADD CONSTRAINT users_tenant_email_key UNIQUE (tenant_id, email),
    ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'member')),
    ADD CONSTRAINT users_tenant_id_key UNIQUE (tenant_id, id);
CREATE POLICY users_isolation ON users
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) = 'admin');
DELETE FROM user_invites WHERE invited_by IS NULL OR role NOT IN ('admin', 'member');
ALTER TABLE user_invites
    DROP CONSTRAINT user_invites_role_check,
    DROP CONSTRAINT user_invites_tenant_fkey,
    DROP CONSTRAINT user_invites_invited_by_fkey,
    ALTER COLUMN invited_by SET NOT NULL,
    ADD CONSTRAINT user_invites_role_check CHECK (role IN ('admin', 'member')),
    ADD CONSTRAINT user_invites_invited_by_fkey FOREIGN KEY (tenant_id, invited_by)
        REFERENCES users (tenant_id, id) ON DELETE CASCADE;
ALTER TABLE vaults DROP CONSTRAINT vaults_tenant_fkey;
DROP TABLE tenant_memberships;
DROP TABLE tenants;
