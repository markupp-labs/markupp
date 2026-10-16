-- +goose Up
-- Quem sai do tenant leva junto o que era só dele lá: os cofres em que era o único
-- membro, com as notas, o pertencimento aos cofres compartilhados e os convites pendentes
-- para o email dele. Fica no banco para valer em qualquer caminho de remoção, e roda como
-- dono porque o administrador que remove não enxerga os cofres do removido.
-- +goose StatementBegin
CREATE FUNCTION markupp_member_left() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    DELETE FROM vaults v
    WHERE v.tenant_id = OLD.tenant_id
      AND EXISTS (SELECT 1 FROM vault_members m WHERE m.vault_id = v.id AND m.user_id = OLD.user_id)
      AND NOT EXISTS (SELECT 1 FROM vault_members m WHERE m.vault_id = v.id AND m.user_id <> OLD.user_id);
    DELETE FROM vault_members m USING vaults v
    WHERE m.vault_id = v.id AND v.tenant_id = OLD.tenant_id AND m.user_id = OLD.user_id;
    UPDATE user_invites i SET declined_at = now()
    FROM users u
    WHERE u.id = OLD.user_id AND i.email = u.email AND i.tenant_id = OLD.tenant_id
      AND i.accepted_at IS NULL AND i.declined_at IS NULL;
    RETURN OLD;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tenant_memberships_member_left
AFTER DELETE ON tenant_memberships
FOR EACH ROW EXECUTE FUNCTION markupp_member_left();

-- +goose Down
DROP TRIGGER IF EXISTS tenant_memberships_member_left ON tenant_memberships;
DROP FUNCTION IF EXISTS markupp_member_left();
