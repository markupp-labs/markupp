-- +goose Up
-- O administrador da instância bloqueia a conta (ADR-0061): ela não ganha credencial nova
-- por nenhum método, e as notas ficam até ele desbloquear ou apagar a conta.
ALTER TABLE users ADD COLUMN blocked_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS blocked_at;
