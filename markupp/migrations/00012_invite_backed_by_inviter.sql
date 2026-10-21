-- +goose Up
-- O convite só vale enquanto quem convidou continua no tenant com papel pelo menos igual
-- ao do convite, e owner ou admin pode revogá-lo antes da resposta.
-- +goose StatementBegin
CREATE FUNCTION markupp_role_rank(role TEXT) RETURNS INT
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE role WHEN 'owner' THEN 4 WHEN 'admin' THEN 3 WHEN 'member' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END
$$;
-- +goose StatementEnd

GRANT DELETE ON user_invites TO markupp_app;

-- +goose Down
REVOKE DELETE ON user_invites FROM markupp_app;
DROP FUNCTION IF EXISTS markupp_role_rank(TEXT);
