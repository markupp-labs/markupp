-- +goose Up
-- Só o owner apaga o tenant, e de vez. O resto vai pelas chaves com ON DELETE CASCADE:
-- vínculos, cofres com notas, fila e índice, convites com notificações. A auditoria não
-- tem chave para o tenant e guarda o registro do apagar.
DROP POLICY tenants_isolation ON tenants;

CREATE POLICY tenants_read ON tenants FOR SELECT
    USING (id = current_setting('markupp.tenant_id', true));

CREATE POLICY tenants_delete ON tenants FOR DELETE
    USING (id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) = 'owner');

GRANT DELETE ON tenants TO markupp_app;

-- +goose Down
REVOKE DELETE ON tenants FROM markupp_app;
DROP POLICY tenants_delete ON tenants;
DROP POLICY tenants_read ON tenants;
CREATE POLICY tenants_isolation ON tenants
    USING (id = current_setting('markupp.tenant_id', true));
