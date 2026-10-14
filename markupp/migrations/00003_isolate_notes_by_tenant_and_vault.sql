-- +goose Up

-- Papel da aplicação nas transações de dados (ADR-0016). Sem login: a conexão troca
-- para ele com SET LOCAL ROLE, e como não é superusuário nem dono das tabelas, o RLS
-- vale sem FORCE. Login e refresh rodam antes de existir tenant e ficam no papel da
-- conexão. O papel é do cluster, então sobrevive a bancos criados em paralelo.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE ROLE markupp_app NOLOGIN NOSUPERUSER NOBYPASSRLS;
EXCEPTION WHEN duplicate_object OR unique_violation THEN
    NULL;
END
$$;
-- +goose StatementEnd

-- Superusuário troca para qualquer papel; outro dono precisa ser membro.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user) THEN
        EXECUTE format('GRANT markupp_app TO %I', current_user);
    END IF;
END
$$;
-- +goose StatementEnd

-- Notas da v1.0.0-rc.1 não tinham cofre. Vão para "Importadas" no tenant padrão, e o
-- primeiro administrador vira membro dele ao ser criado.
INSERT INTO vaults (id, tenant_id, name, created_at)
SELECT 'importadas', 'default', 'Importadas', now()
WHERE EXISTS (SELECT 1 FROM notes);

ALTER TABLE vaults ADD CONSTRAINT vaults_tenant_id_key UNIQUE (tenant_id, id);

ALTER TABLE notes ADD COLUMN tenant_id TEXT, ADD COLUMN vault_id TEXT;
UPDATE notes SET tenant_id = 'default', vault_id = 'importadas';
ALTER TABLE notes
    ALTER COLUMN tenant_id SET NOT NULL,
    ALTER COLUMN tenant_id SET DEFAULT current_setting('markupp.tenant_id', true),
    ALTER COLUMN vault_id SET NOT NULL,
    ADD CONSTRAINT notes_vault_fkey FOREIGN KEY (tenant_id, vault_id)
        REFERENCES vaults (tenant_id, id) ON DELETE CASCADE,
    DROP CONSTRAINT notes_path_key,
    ADD CONSTRAINT notes_vault_path_key UNIQUE (vault_id, path);

-- Cofres do usuário da sessão. SECURITY DEFINER para a política de vault_members
-- consultar a própria tabela sem recursão.
-- +goose StatementBegin
CREATE FUNCTION markupp_member_vaults() RETURNS SETOF TEXT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
    SELECT vault_id FROM vault_members
    WHERE user_id = current_setting('markupp.user_id', true)
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_member_vaults() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_member_vaults() TO markupp_app;

ALTER TABLE notes ENABLE ROW LEVEL SECURITY;
CREATE POLICY notes_isolation ON notes
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id IN (SELECT markupp_member_vaults()));

ALTER TABLE vaults ENABLE ROW LEVEL SECURITY;
CREATE POLICY vaults_isolation ON vaults
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND id IN (SELECT markupp_member_vaults()));

ALTER TABLE vault_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY vault_members_isolation ON vault_members
    USING (vault_id IN (SELECT markupp_member_vaults()));

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_isolation ON users
    USING (tenant_id = current_setting('markupp.tenant_id', true));

ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY audit_events_isolation ON audit_events
    USING (tenant_id = current_setting('markupp.tenant_id', true));

GRANT USAGE ON SCHEMA public TO markupp_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON notes TO markupp_app;
GRANT SELECT ON vaults, vault_members, users TO markupp_app;
GRANT INSERT ON audit_events TO markupp_app;

-- +goose Down
-- Destrutivo: notas de cofres diferentes com o mesmo caminho impedem a volta do
-- UNIQUE (path), e a migração falha até que sejam renomeadas.
REVOKE ALL ON notes, vaults, vault_members, users, audit_events FROM markupp_app;
REVOKE USAGE ON SCHEMA public FROM markupp_app;
DROP POLICY IF EXISTS audit_events_isolation ON audit_events;
DROP POLICY IF EXISTS users_isolation ON users;
DROP POLICY IF EXISTS vault_members_isolation ON vault_members;
DROP POLICY IF EXISTS vaults_isolation ON vaults;
DROP POLICY IF EXISTS notes_isolation ON notes;
ALTER TABLE audit_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE vault_members DISABLE ROW LEVEL SECURITY;
ALTER TABLE vaults DISABLE ROW LEVEL SECURITY;
ALTER TABLE notes DISABLE ROW LEVEL SECURITY;
DROP FUNCTION IF EXISTS markupp_member_vaults();
ALTER TABLE notes
    DROP CONSTRAINT notes_vault_path_key,
    DROP CONSTRAINT notes_vault_fkey,
    ADD CONSTRAINT notes_path_key UNIQUE (path),
    DROP COLUMN vault_id,
    DROP COLUMN tenant_id;
ALTER TABLE vaults DROP CONSTRAINT vaults_tenant_id_key;
