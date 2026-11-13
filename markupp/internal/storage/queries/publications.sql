-- Publicações (ADR-0048), no papel da aplicação e sob RLS (00040): publicar a nota
-- exige poder editá-la; pasta e cofre, owner ou admin membro do cofre.

-- Pasta vazia conta o cofre inteiro.
-- name: CountNotesUnder :one
SELECT count(*) FROM notes
WHERE vault_id = sqlc.arg(vault_id)
  AND (sqlc.arg(folder_path)::text = '' OR starts_with(path, sqlc.arg(folder_path)::text || '/'));

-- name: GetVaultName :one
SELECT name FROM vaults WHERE id = sqlc.arg(id);

-- O tenant vem do DEFAULT, lido da sessão; o gatilho cobre as notas na mesma instrução.
-- name: InsertPublication :exec
INSERT INTO publications (id, vault_id, kind, note_id, folder_path, indexable, published_by, published_at)
VALUES (sqlc.arg(id), sqlc.arg(vault_id), sqlc.arg(kind), sqlc.narg(note_id), sqlc.narg(folder_path),
        sqlc.arg(indexable), sqlc.arg(published_by), sqlc.arg(published_at));

-- name: FindTargetPublication :one
SELECT id FROM publications
WHERE vault_id = sqlc.arg(vault_id) AND kind = sqlc.arg(kind)
  AND note_id IS NOT DISTINCT FROM sqlc.narg(note_id)
  AND folder_path IS NOT DISTINCT FROM sqlc.narg(folder_path);

-- Lista pela função, que traz os nomes que o RLS das notas e dos cofres esconderia do
-- owner e do admin fora do cofre. Cofre e id vazios não filtram.
-- name: ListPublications :many
SELECT id::text AS id, vault_id::text AS vault_id, vault_name::text AS vault_name, kind::text AS kind,
       COALESCE(note_id, '')::text AS note_id, COALESCE(note_path, '')::text AS note_path,
       COALESCE(folder_path, '')::text AS folder_path, note_count::bigint AS note_count,
       indexable::bool AS indexable, COALESCE(public_id, '')::text AS public_id,
       published_by::text AS published_by, COALESCE(published_by_email, '')::text AS published_by_email,
       published_at::timestamptz AS published_at
FROM markupp_list_publications(sqlc.arg(vault_id)::text, sqlc.arg(id)::text);

-- name: SetPublicationIndexable :execrows
UPDATE publications SET indexable = sqlc.arg(indexable) WHERE id = sqlc.arg(id);

-- name: DeletePublication :execrows
DELETE FROM publications WHERE id = sqlc.arg(id);

-- name: PublicationVisible :one
SELECT EXISTS (SELECT 1 FROM publications WHERE id = sqlc.arg(id));
