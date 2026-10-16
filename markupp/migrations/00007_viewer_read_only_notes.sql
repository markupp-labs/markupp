-- +goose Up
-- O viewer lê as notas dos cofres em que está e não escreve nelas. A rota barra antes,
-- e o banco barra de novo.
DROP POLICY notes_isolation ON notes;

CREATE POLICY notes_read ON notes FOR SELECT
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id IN (SELECT markupp_member_vaults()));

CREATE POLICY notes_insert ON notes FOR INSERT
    WITH CHECK (tenant_id = current_setting('markupp.tenant_id', true)
                AND vault_id IN (SELECT markupp_member_vaults())
                AND current_setting('markupp.role', true) <> 'viewer');

CREATE POLICY notes_update ON notes FOR UPDATE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) <> 'viewer');

CREATE POLICY notes_delete ON notes FOR DELETE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) <> 'viewer');

-- +goose Down
DROP POLICY notes_delete ON notes;
DROP POLICY notes_update ON notes;
DROP POLICY notes_insert ON notes;
DROP POLICY notes_read ON notes;
CREATE POLICY notes_isolation ON notes
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND vault_id IN (SELECT markupp_member_vaults()));
