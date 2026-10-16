-- Pedidos de email da conta, no papel da conexão: a conta é da instância, fora do
-- recorte por tenant. O token nunca passa por aqui, só o hash dele.

-- name: LockUserEmailVerified :one
SELECT email_verified FROM users WHERE id = $1 FOR UPDATE;

-- name: RecentEmailVerificationExists :one
SELECT EXISTS (
    SELECT 1 FROM email_verifications
    WHERE user_id = sqlc.arg(user_id) AND used_at IS NULL AND created_at > sqlc.arg(since)
);

-- name: DeleteOpenEmailVerifications :exec
DELETE FROM email_verifications WHERE user_id = $1 AND used_at IS NULL;

-- name: CreateEmailVerification :one
INSERT INTO email_verifications (user_id, created_at) VALUES ($1, $2) RETURNING id;

-- name: EnqueueEmailVerificationNotification :exec
INSERT INTO notifications (tenant_id, email_verification_id) VALUES (NULL, $1);

-- name: UseEmailVerification :one
UPDATE email_verifications SET used_at = sqlc.arg(now)::timestamptz
WHERE token_hash = sqlc.arg(token_hash) AND used_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING user_id;

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified = TRUE WHERE id = $1;

-- name: LocalUserIDByEmail :one
SELECT user_id FROM user_identities WHERE provider = 'local' AND provider_user_id = $1;

-- name: RecentPasswordResetExists :one
SELECT EXISTS (
    SELECT 1 FROM password_resets
    WHERE user_id = sqlc.arg(user_id) AND used_at IS NULL AND created_at > sqlc.arg(since)
);

-- name: DeleteOpenPasswordResets :exec
DELETE FROM password_resets WHERE user_id = $1 AND used_at IS NULL;

-- name: CreatePasswordReset :one
INSERT INTO password_resets (user_id, created_at) VALUES ($1, $2) RETURNING id;

-- name: EnqueuePasswordResetNotification :exec
INSERT INTO notifications (tenant_id, password_reset_id) VALUES (NULL, $1);

-- name: UsePasswordReset :one
UPDATE password_resets SET used_at = sqlc.arg(now)::timestamptz
WHERE token_hash = sqlc.arg(token_hash) AND used_at IS NULL AND expires_at > sqlc.arg(now)
RETURNING user_id;

-- name: UpdateLocalPassword :exec
UPDATE user_identities SET password_hash = $2 WHERE user_id = $1 AND provider = 'local';
