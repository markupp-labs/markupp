-- +goose Up
-- Com a conta em vários tenants, os cofres do usuário na sessão são só os do tenant em
-- que ela age. Todas as políticas de cofre e de nota passam por esta função.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_member_vaults() RETURNS SETOF TEXT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT m.vault_id FROM vault_members m JOIN vaults v ON v.id = m.vault_id
    WHERE m.user_id = current_setting('markupp.user_id', true)
      AND v.tenant_id = current_setting('markupp.tenant_id', true)
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION markupp_member_vaults() RETURNS SETOF TEXT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT vault_id FROM vault_members
    WHERE user_id = current_setting('markupp.user_id', true)
$$;
-- +goose StatementEnd
