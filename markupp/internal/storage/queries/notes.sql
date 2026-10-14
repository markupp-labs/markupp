-- name: CreateNote :exec
INSERT INTO notes (id, path, content, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetNoteByID :one
SELECT id, path, content, created_at, updated_at FROM notes WHERE id = $1;

-- name: ListNotes :many
SELECT id, path, content, created_at, updated_at FROM notes
ORDER BY path;

-- name: UpdateNoteWithVersionCheck :one
UPDATE notes
SET path = sqlc.arg(path), content = sqlc.arg(content), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND updated_at = sqlc.arg(prev_updated_at)
RETURNING id, path, content, created_at, updated_at;

-- name: UpdateNoteForced :one
UPDATE notes
SET path = $1, content = $2, updated_at = $3
WHERE id = $4
RETURNING id, path, content, created_at, updated_at;

-- name: DeleteNote :execrows
DELETE FROM notes WHERE id = $1;

-- name: SearchNotes :many
SELECT id, path, updated_at FROM notes
WHERE content ILIKE $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;
