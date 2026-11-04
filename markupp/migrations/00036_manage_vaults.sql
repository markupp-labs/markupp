-- +goose Up
-- O membro do cofre que não é viewer cria, renomeia e apaga cofre e escolhe quem é
-- membro, entre as contas do tenant; o viewer só sai (ADR-0024). As políticas FOR ALL
-- viram leitura, porque políticas permissivas somam e deixariam o viewer escrever.
DROP POLICY vaults_isolation ON vaults;
CREATE POLICY vaults_read ON vaults FOR SELECT
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND id IN (SELECT markupp_member_vaults()));
CREATE POLICY vaults_update ON vaults FOR UPDATE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) <> 'viewer');
CREATE POLICY vaults_delete ON vaults FOR DELETE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) <> 'viewer');

DROP POLICY vault_members_isolation ON vault_members;
CREATE POLICY vault_members_read ON vault_members FOR SELECT
    USING (vault_id IN (SELECT markupp_member_vaults()));
CREATE POLICY vault_members_insert ON vault_members FOR INSERT
    WITH CHECK (vault_id IN (SELECT markupp_member_vaults())
                AND current_setting('markupp.role', true) <> 'viewer'
                AND user_id IN (SELECT user_id FROM tenant_memberships
                                WHERE tenant_id = current_setting('markupp.tenant_id', true)));
CREATE POLICY vault_members_delete ON vault_members FOR DELETE
    USING (vault_id IN (SELECT markupp_member_vaults())
           AND (current_setting('markupp.role', true) <> 'viewer'
                OR user_id = current_setting('markupp.user_id', true)));

-- O cofre novo ainda não é de ninguém, então a política de INSERT não aceitaria o primeiro
-- membro. A função grava os dois como dono, e o papel vem do vínculo no banco.
-- +goose StatementBegin
CREATE FUNCTION markupp_create_vault(p_id TEXT, p_name TEXT, p_created_at TIMESTAMPTZ) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    v_tenant TEXT := current_setting('markupp.tenant_id', true);
    v_user   TEXT := current_setting('markupp.user_id', true);
BEGIN
    IF NOT EXISTS (SELECT 1 FROM tenant_memberships
                   WHERE tenant_id = v_tenant AND user_id = v_user AND role <> 'viewer') THEN
        RAISE EXCEPTION 'conta % sem vinculo de escrita no tenant %', v_user, v_tenant
            USING ERRCODE = 'insufficient_privilege';
    END IF;
    INSERT INTO vaults (id, tenant_id, name, created_at) VALUES (p_id, v_tenant, p_name, p_created_at);
    INSERT INTO vault_members (vault_id, user_id, created_at) VALUES (p_id, v_user, p_created_at);
END;
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_create_vault(TEXT, TEXT, TIMESTAMPTZ) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_create_vault(TEXT, TEXT, TIMESTAMPTZ) TO markupp_app;

GRANT UPDATE, DELETE ON vaults TO markupp_app;
GRANT INSERT, DELETE ON vault_members TO markupp_app;

-- +goose Down
REVOKE INSERT, DELETE ON vault_members FROM markupp_app;
REVOKE UPDATE, DELETE ON vaults FROM markupp_app;
DROP FUNCTION IF EXISTS markupp_create_vault(TEXT, TEXT, TIMESTAMPTZ);
DROP POLICY vault_members_delete ON vault_members;
DROP POLICY vault_members_insert ON vault_members;
DROP POLICY vault_members_read ON vault_members;
CREATE POLICY vault_members_isolation ON vault_members
    USING (vault_id IN (SELECT markupp_member_vaults()));
DROP POLICY vaults_delete ON vaults;
DROP POLICY vaults_update ON vaults;
DROP POLICY vaults_read ON vaults;
CREATE POLICY vaults_isolation ON vaults
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND id IN (SELECT markupp_member_vaults()));
