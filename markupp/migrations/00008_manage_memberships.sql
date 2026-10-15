-- +goose Up
-- Owner e admin trocam papel e removem membro. A regra de quem mexe em quem fica na
-- aplicação; o banco garante que só owner e admin escrevem nos vínculos.
DROP POLICY tenant_memberships_isolation ON tenant_memberships;

CREATE POLICY tenant_memberships_read ON tenant_memberships FOR SELECT
    USING (tenant_id = current_setting('markupp.tenant_id', true));

CREATE POLICY tenant_memberships_update ON tenant_memberships FOR UPDATE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) IN ('owner', 'admin'));

CREATE POLICY tenant_memberships_delete ON tenant_memberships FOR DELETE
    USING (tenant_id = current_setting('markupp.tenant_id', true)
           AND current_setting('markupp.role', true) IN ('owner', 'admin'));

GRANT UPDATE (role), DELETE ON tenant_memberships TO markupp_app;

-- +goose Down
REVOKE UPDATE (role), DELETE ON tenant_memberships FROM markupp_app;
DROP POLICY tenant_memberships_delete ON tenant_memberships;
DROP POLICY tenant_memberships_update ON tenant_memberships;
DROP POLICY tenant_memberships_read ON tenant_memberships;
CREATE POLICY tenant_memberships_isolation ON tenant_memberships
    USING (tenant_id = current_setting('markupp.tenant_id', true));
