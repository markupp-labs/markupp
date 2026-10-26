-- Publicação do cofre (ADR-0048), no papel da aplicação e sob RLS: só owner e admin
-- membros do cofre escrevem, e quem é membro dele lê.

-- O cofre vem da leitura de vaults, recortada pelo RLS ao que o ator enxerga: cofre
-- fora do alcance não insere nada. Título vazio fica com o nome do cofre. Publicar de
-- novo troca o slug e o título e mantém quem publicou primeiro e quando.
-- name: UpsertVaultPublication :one
INSERT INTO vault_publications (vault_id, slug, title, published_by, published_at)
SELECT v.id, sqlc.arg(slug)::text, COALESCE(NULLIF(sqlc.arg(title)::text, ''), v.name),
       sqlc.arg(published_by)::text, sqlc.arg(published_at)::timestamptz
FROM vaults v
WHERE v.id = sqlc.arg(vault_id)::text
ON CONFLICT (vault_id) DO UPDATE SET slug = EXCLUDED.slug, title = EXCLUDED.title
RETURNING vault_id, slug, title, published_by, published_at;

-- name: DeleteVaultPublication :execrows
DELETE FROM vault_publications WHERE vault_id = $1;

-- name: GetVaultPublication :one
SELECT vault_id, slug, title, published_by, published_at
FROM vault_publications
WHERE vault_id = $1;

-- Lida no papel markupp_reader, que só tem grant nestas colunas.
-- name: GetPublishedVault :one
SELECT vault_id, slug, title, published_at
FROM vault_publications
WHERE slug = $1;
