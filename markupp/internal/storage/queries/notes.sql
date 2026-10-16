-- O tenant da nota vem do DEFAULT da coluna, lido da sessão, e o RLS recorta toda
-- leitura e escrita ao tenant e aos cofres do usuário: nenhuma query repete o filtro.

-- name: CreateNote :exec
INSERT INTO notes (id, vault_id, path, content, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: DefaultVaultID :one
SELECT vault_id FROM vault_members
WHERE user_id = current_setting('markupp.user_id', true)
ORDER BY created_at, vault_id
LIMIT 1;

-- name: GetNoteByID :one
SELECT * FROM notes WHERE id = $1;

-- name: ListNotes :many
SELECT * FROM notes
ORDER BY path, vault_id;

-- name: UpdateNoteWithVersionCheck :one
UPDATE notes
SET path = sqlc.arg(path), content = sqlc.arg(content), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND updated_at = sqlc.arg(prev_updated_at)
RETURNING *;

-- name: UpdateNoteForced :one
UPDATE notes
SET path = $1, content = $2, updated_at = $3
WHERE id = $4
RETURNING *;

-- name: DeleteNote :execrows
DELETE FROM notes WHERE id = $1;

-- name: SearchNotes :many
SELECT id, vault_id, path, updated_at FROM notes
WHERE content ILIKE $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;
