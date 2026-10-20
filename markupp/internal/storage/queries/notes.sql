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
-- Cofre vazio busca em todos os cofres do ator.
-- name: SearchNotes :many
SELECT n.id, n.vault_id, n.path, n.updated_at,
       ts_headline(sqlc.arg(search_config)::text::regconfig, n.content, q.query,
                   'StartSel=' || chr(2) || ', StopSel=' || chr(3))::text AS snippet,
       ts_rank(s.document, q.query, 1)::float8 AS rank
FROM websearch_to_tsquery(sqlc.arg(search_config)::text::regconfig, sqlc.arg(query)::text) AS q (query)
JOIN note_search_index s ON s.document @@ q.query
JOIN notes n ON n.id = s.note_id
WHERE sqlc.arg(vault_id)::text = '' OR n.vault_id = sqlc.arg(vault_id)::text
ORDER BY rank DESC, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- Organização derivada da nota, nula antes do segundo estágio.
-- name: GetNoteRank :one
SELECT c.pagerank, c.in_degree, c.out_degree, m.community_id
FROM notes n
LEFT JOIN note_centrality c ON c.note_id = n.id
LEFT JOIN note_communities m ON m.note_id = n.id
WHERE n.id = sqlc.arg(note_id);

-- A nota sem organização derivada ainda não passou pelo segundo estágio: entra em
-- orphan=false, como no grafo, fica fora do filtro de comunidade e vai para o fim das
-- ordens por PageRank e por grau. orphan=true vai por ListVaultOrphanNotes.
-- name: ListVaultNotes :many
SELECT n.id, n.vault_id, n.path, n.content, n.content_sha256, n.created_at, n.updated_at,
       c.pagerank, c.in_degree, c.out_degree, m.community_id
FROM notes n
LEFT JOIN note_centrality c ON c.note_id = n.id
LEFT JOIN note_communities m ON m.note_id = n.id
WHERE n.vault_id = sqlc.arg(vault_id)
  AND (sqlc.narg(orphan)::bool IS NULL OR COALESCE(c.in_degree + c.out_degree = 0, false) = sqlc.narg(orphan)::bool)
  AND (sqlc.narg(community_id)::bigint IS NULL OR m.community_id = sqlc.narg(community_id)::bigint)
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'pagerank' AND sqlc.arg(descending)::bool THEN c.pagerank END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'pagerank' AND NOT sqlc.arg(descending)::bool THEN c.pagerank END NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'degree' AND sqlc.arg(descending)::bool THEN c.in_degree + c.out_degree END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'degree' AND NOT sqlc.arg(descending)::bool THEN c.in_degree + c.out_degree END NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'updated' AND sqlc.arg(descending)::bool THEN n.updated_at END DESC,
    CASE WHEN sqlc.arg(sort)::text = 'updated' AND NOT sqlc.arg(descending)::bool THEN n.updated_at END,
    n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- As órfãs do cofre com o predicado fixo do índice parcial, que o plano genérico de
-- ListVaultNotes não usa porque lá o filtro depende do parâmetro. O LEFT JOIN mantém
-- as colunas da linha iguais às de ListVaultNotes; o filtro em c o torna interno.
-- name: ListVaultOrphanNotes :many
SELECT n.id, n.vault_id, n.path, n.content, n.content_sha256, n.created_at, n.updated_at,
       c.pagerank, c.in_degree, c.out_degree, m.community_id
FROM notes n
LEFT JOIN note_centrality c ON c.note_id = n.id
LEFT JOIN note_communities m ON m.note_id = n.id
WHERE c.vault_id = sqlc.arg(vault_id) AND c.in_degree = 0 AND c.out_degree = 0
  AND (sqlc.narg(community_id)::bigint IS NULL OR m.community_id = sqlc.narg(community_id)::bigint)
ORDER BY
    CASE WHEN sqlc.arg(sort)::text = 'pagerank' AND sqlc.arg(descending)::bool THEN c.pagerank END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'pagerank' AND NOT sqlc.arg(descending)::bool THEN c.pagerank END NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'degree' AND sqlc.arg(descending)::bool THEN c.in_degree + c.out_degree END DESC NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'degree' AND NOT sqlc.arg(descending)::bool THEN c.in_degree + c.out_degree END NULLS LAST,
    CASE WHEN sqlc.arg(sort)::text = 'updated' AND sqlc.arg(descending)::bool THEN n.updated_at END DESC,
    CASE WHEN sqlc.arg(sort)::text = 'updated' AND NOT sqlc.arg(descending)::bool THEN n.updated_at END,
    n.path, n.id
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);
