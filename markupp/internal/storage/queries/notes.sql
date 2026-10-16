-- O tenant da nota vem do DEFAULT da coluna, lido da sessão, e o RLS recorta toda
-- leitura e escrita ao tenant e aos cofres do usuário: nenhuma query repete o filtro.

-- name: CreateNote :one
INSERT INTO notes (id, vault_id, path, content, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

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

-- Lê só o índice: a nota ainda não indexada fica fora (ADR-0012). Consulta sem lexema,
-- vazia ou só de stopwords, não casa com nada. O id desempata o ranking para a
-- paginação ser estável. O trecho marca os termos com U+0002 e U+0003, e não com
-- HTML, para o cliente escapar o texto antes de trocar os marcadores por destaque.
-- name: SearchNotes :many
SELECT n.id, n.vault_id, n.path, n.updated_at,
       ts_headline(sqlc.arg(search_config)::text::regconfig, n.content, q.query,
                   'StartSel=' || chr(2) || ', StopSel=' || chr(3))::text AS snippet
FROM websearch_to_tsquery(sqlc.arg(search_config)::text::regconfig, sqlc.arg(query)::text) AS q (query)
JOIN note_search_index s ON s.document @@ q.query
JOIN notes n ON n.id = s.note_id
ORDER BY ts_rank(s.document, q.query, 1) DESC, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
