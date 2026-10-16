-- +goose Up
-- Convite em handshake: a conta convidada aceita ou recusa, e recusa também encerra o
-- token de cadastro.
ALTER TABLE user_invites ADD COLUMN declined_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE user_invites DROP COLUMN declined_at;
