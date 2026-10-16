-- Pedidos de email da conta, no papel da conexão: a conta é da instância, fora do
-- recorte por tenant. O token nunca passa por aqui, só o hash dele. A trava na linha da
-- conta serializa os pedidos dela, e o intervalo entre pedidos vale mesmo em paralelo.

-- name: LockUserEmailVerified :one
SELECT email_verified FROM users WHERE id = $1 FOR UPDATE;

-- name: RecentAccountTokenExists :one
SELECT EXISTS (
    SELECT 1 FROM account_tokens
    WHERE user_id = sqlc.arg(user_id) AND purpose = sqlc.arg(purpose) AND used_at IS NULL
      AND created_at > now() - sqlc.arg(cooldown)::interval
);

-- name: DeleteOpenAccountTokens :exec
DELETE FROM account_tokens WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL;

-- name: CreateAccountToken :one
INSERT INTO account_tokens (user_id, purpose) VALUES ($1, $2) RETURNING id;

-- name: EnqueueAccountTokenNotification :exec
INSERT INTO notifications (tenant_id, kind, account_token_id)
VALUES (NULL, sqlc.arg(kind), sqlc.arg(account_token_id));

-- Sem user_id, vale o token de qualquer conta, como na troca de senha.
-- name: UseAccountToken :one
UPDATE account_tokens SET used_at = now()
WHERE token_hash = sqlc.arg(token_hash) AND purpose = sqlc.arg(purpose)
  AND user_id = COALESCE(sqlc.narg(user_id), user_id)
  AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified = TRUE WHERE id = $1;

-- name: LocalUserIDByEmail :one
SELECT user_id FROM user_identities WHERE provider = 'local' AND provider_user_id = $1;

-- name: UpdateLocalPassword :exec
UPDATE user_identities SET password_hash = $2 WHERE user_id = $1 AND provider = 'local';
