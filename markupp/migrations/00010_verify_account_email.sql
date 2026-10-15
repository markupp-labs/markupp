-- +goose Up
-- Sem envio de email (ADR-0021), só o primeiro acesso da instância e o cadastro pelo token
-- de convite provam o email. Conta de auto-registro nasce sem essa prova e fica fora do
-- handshake de convites, senão quem cadastrasse o email de outra pessoa tomaria o convite.
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose Down
ALTER TABLE users DROP COLUMN email_verified;
