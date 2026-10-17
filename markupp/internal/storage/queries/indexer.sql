-- O indexador reserva o cofre no papel da conexão, que enxerga todos os tenants, e
-- só então entra no papel markupp_indexer, preso pelo RLS ao cofre reservado.

-- Trava a linha do cofre, e não a fila, para dois Jobs nunca pegarem o mesmo cofre. NO KEY
-- UPDATE não conflita com a checagem de chave estrangeira, então as notas seguem
-- graváveis. A ordem aleatória impede que um cofre que sempre falha segure os outros.
-- name: ReservePendingVault :one
SELECT v.id AS vault_id, v.tenant_id
FROM vaults v
WHERE v.id IN (SELECT n.vault_id FROM note_index_queue q JOIN notes n ON n.id = q.note_id)
ORDER BY random()
LIMIT 1
FOR NO KEY UPDATE OF v SKIP LOCKED;

-- name: EnterIndexer :exec
SELECT set_config('role', 'markupp_indexer', true),
       set_config('markupp.tenant_id', sqlc.arg(tenant_id)::text, true),
       set_config('markupp.vault_id', sqlc.arg(vault_id)::text, true);

-- O filtro de cofre repete o RLS para a consulta usar o índice de notes por cofre.
-- name: ListPendingNotes :many
SELECT n.id, n.path, n.content, q.revision
FROM note_index_queue q
JOIN notes n ON n.id = q.note_id
WHERE n.vault_id = sqlc.arg(vault_id)
ORDER BY n.id;

-- Nota apagada depois da leitura some no JOIN e não é gravada. O título pesa A e o
-- corpo pesa D no ts_rank. O corte em 250 mil caracteres do corpo (até 1MB em UTF-8)
-- mantém o tsvector abaixo do teto de 1MB de lexemas.
-- name: SaveSearchDocuments :exec
INSERT INTO note_search_index (note_id, document)
SELECT n.id, setweight(to_tsvector(sqlc.arg(search_config)::text::regconfig, d.title), 'A')
          || setweight(to_tsvector(sqlc.arg(search_config)::text::regconfig, left(d.body, 250000)), 'D')
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(titles)::text[]) AS title,
             unnest(sqlc.arg(bodies)::text[]) AS body) d
JOIN notes n ON n.id = d.note_id
ON CONFLICT (note_id) DO UPDATE SET document = EXCLUDED.document;

-- Tira da fila só a revisão lida: a escrita confirmada depois dela continua pendente.
-- name: DequeueNotes :exec
DELETE FROM note_index_queue q
USING (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(revisions)::bigint[]) AS revision) r
WHERE q.note_id = r.note_id AND q.revision = r.revision;

-- Resolve o nome pelo mesmo cast que a gravação usa, com o search_path da conexão.
-- name: ResolveSearchConfig :one
SELECT sqlc.arg(name)::text::regconfig::oid::bigint AS config_oid;

-- Roda no papel da conexão, o dono do banco: o NULL reenfileira a instalação inteira.
-- name: RequeueNotes :one
SELECT markupp_requeue_notes(sqlc.narg(vault_id)::text) AS requeued;

-- O lote substitui as linhas do grafo das notas dele: apaga e grava de novo.
-- name: DeleteNoteGraph :exec
WITH apagadas AS (DELETE FROM note_links WHERE note_id = ANY (sqlc.arg(note_ids)::text[]))
DELETE FROM note_titles WHERE note_id = ANY (sqlc.arg(note_ids)::text[]);

-- O cofre vem da nota. Nota apagada depois da leitura some no JOIN.
-- name: SaveNoteTitles :exec
INSERT INTO note_titles (note_id, vault_id, title_key)
SELECT n.id, n.vault_id, d.title_key
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(title_keys)::text[]) AS title_key) d
JOIN notes n ON n.id = d.note_id;

-- Caminho vazio é ligação sem caminho, isto é, que não veio de link markdown.
-- name: SaveNoteLinks :exec
INSERT INTO note_links (note_id, vault_id, cited_title, title_key, target_path, kind)
SELECT n.id, n.vault_id, d.cited_title, d.title_key, NULLIF(d.target_path, ''), d.kind
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(cited_titles)::text[]) AS cited_title,
             unnest(sqlc.arg(title_keys)::text[]) AS title_key, unnest(sqlc.arg(target_paths)::text[]) AS target_path,
             unnest(sqlc.arg(kinds)::text[]) AS kind) d
JOIN notes n ON n.id = d.note_id;
