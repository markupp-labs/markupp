-- O indexador de embedding escolhe o cofre no papel da conexão e lê e grava no papel
-- markupp_indexer, preso pelo RLS ao cofre escolhido, como o indexador de texto. Não
-- reserva o cofre: o modelo roda fora da transação, e a gravação confere o hash.

-- name: PickEmbeddingVault :one
SELECT v.id AS vault_id, v.tenant_id
FROM vaults v
WHERE v.id IN (SELECT n.vault_id FROM note_embedding_queue q JOIN notes n ON n.id = q.note_id)
ORDER BY random()
LIMIT 1;

-- name: ListPendingEmbeddingNotes :many
SELECT n.id, n.content, n.content_sha256, q.revision
FROM note_embedding_queue q
JOIN notes n ON n.id = q.note_id
WHERE n.vault_id = sqlc.arg(vault_id)
ORDER BY q.revision
LIMIT sqlc.arg(batch_limit);

-- name: DeleteNoteEmbeddings :exec
DELETE FROM note_embeddings WHERE note_id = ANY (sqlc.arg(note_ids)::text[]);

-- O vetor vem como texto do pgvector, [x,y,...]. Só grava a nota que ainda tem o texto
-- lido: a que mudou depois continua na fila com outra revisão.
-- name: SaveNoteEmbeddings :exec
INSERT INTO note_embeddings (note_id, vault_id, embedding)
SELECT n.id, n.vault_id, d.embedding::vector
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(content_hashes)::bytea[]) AS content_hash,
             unnest(sqlc.arg(embeddings)::text[]) AS embedding) d
JOIN notes n ON n.id = d.note_id AND n.content_sha256 = d.content_hash;

-- name: DequeueEmbeddingNotes :exec
DELETE FROM note_embedding_queue q
USING (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(revisions)::bigint[]) AS revision) r
WHERE q.note_id = r.note_id AND q.revision = r.revision;

-- Roda no papel da conexão, o dono do banco.
-- name: UseEmbeddingModel :one
SELECT markupp_use_embedding_model(sqlc.arg(name)::text, sqlc.arg(dimension)::int)::bool AS changed;
