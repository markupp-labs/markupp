-- +goose Up
-- Publicação de cofre (ADR-0048). A publicação é a linha: existir é estar publicado. O
-- slug é global entre tenants, porque a URL pública não leva o tenant. O tenant vem do
-- DEFAULT, lido da sessão, e a chave com o cofre impede publicar cofre de outro tenant.
CREATE TABLE vault_publications (
    vault_id     TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL DEFAULT current_setting('markupp.tenant_id', true),
    slug         TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]{3,63}$'),
    title        TEXT NOT NULL,
    published_by TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT vault_publications_vault_fkey FOREIGN KEY (tenant_id, vault_id)
        REFERENCES vaults (tenant_id, id) ON DELETE CASCADE ON UPDATE CASCADE
);

-- Papel da leitura sem conta: só lê, e só o que está nos cofres publicados. A API troca
-- para ele com SET LOCAL ROLE numa transação própria, paralela à do ator.
-- +goose StatementBegin
DO $$
BEGIN
    CREATE ROLE markupp_reader NOLOGIN NOSUPERUSER NOBYPASSRLS;
EXCEPTION WHEN duplicate_object OR unique_violation THEN
    NULL;
END
$$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user) THEN
        EXECUTE format('GRANT markupp_reader TO %I', current_user);
    END IF;
END
$$;
-- +goose StatementEnd

-- Cofres publicados. SECURITY DEFINER para as políticas do leitor não dependerem do
-- grant dele em vault_publications.
-- +goose StatementBegin
CREATE FUNCTION markupp_public_vaults() RETURNS SETOF TEXT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT vault_id FROM vault_publications
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_public_vaults() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_public_vaults() TO markupp_reader;

-- Publicar e despublicar é do owner e do admin do tenant, no cofre de que são membros;
-- quem é membro do cofre lê a publicação dele.
GRANT SELECT, INSERT, DELETE ON vault_publications TO markupp_app;
GRANT UPDATE (slug, title) ON vault_publications TO markupp_app;
ALTER TABLE vault_publications ENABLE ROW LEVEL SECURITY;
CREATE POLICY vault_publications_read ON vault_publications FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));
CREATE POLICY vault_publications_insert ON vault_publications FOR INSERT TO markupp_app
    WITH CHECK (tenant_id = current_setting('markupp.tenant_id', true)
                AND vault_id IN (SELECT markupp_member_vaults())
                AND current_setting('markupp.role', true) IN ('owner', 'admin'));
CREATE POLICY vault_publications_update ON vault_publications FOR UPDATE TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) IN ('owner', 'admin'));
CREATE POLICY vault_publications_delete ON vault_publications FOR DELETE TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults())
           AND current_setting('markupp.role', true) IN ('owner', 'admin'));

-- O leitor só recebe SELECT, e só nas tabelas e views de que as rotas públicas leem: a
-- nota, o índice léxico, as ligações, a organização derivada e a versão do grafo. Da
-- publicação, só o que a URL pública mostra: nem tenant nem quem publicou.
GRANT USAGE ON SCHEMA public TO markupp_reader;
GRANT SELECT (vault_id, slug, title, published_at) ON vault_publications TO markupp_reader;
GRANT SELECT ON notes, note_search_index, note_titles, note_links, note_centrality,
    note_communities, vault_graph_versions TO markupp_reader;
GRANT SELECT ON note_edges, note_link_items, note_unresolved_link_items TO markupp_reader;
-- As políticas de notes sem papel (00007) valem para todos e chamam esta função, que
-- para o leitor, sem usuário na sessão, não devolve nada.
GRANT EXECUTE ON FUNCTION markupp_member_vaults() TO markupp_reader;

CREATE POLICY vault_publications_public ON vault_publications FOR SELECT TO markupp_reader
    USING (true);
CREATE POLICY notes_public ON notes FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));
-- A linha do índice herda o recorte de notes, como em note_search_index_read.
CREATE POLICY note_search_index_public ON note_search_index FOR SELECT TO markupp_reader
    USING (EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_titles_public ON note_titles FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));
CREATE POLICY note_links_public ON note_links FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));
CREATE POLICY note_centrality_public ON note_centrality FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));
CREATE POLICY note_communities_public ON note_communities FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));
CREATE POLICY vault_graph_versions_public ON vault_graph_versions FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT markupp_public_vaults()));

-- +goose Down
DROP POLICY IF EXISTS vault_graph_versions_public ON vault_graph_versions;
DROP POLICY IF EXISTS note_communities_public ON note_communities;
DROP POLICY IF EXISTS note_centrality_public ON note_centrality;
DROP POLICY IF EXISTS note_links_public ON note_links;
DROP POLICY IF EXISTS note_titles_public ON note_titles;
DROP POLICY IF EXISTS note_search_index_public ON note_search_index;
DROP POLICY IF EXISTS notes_public ON notes;
REVOKE EXECUTE ON FUNCTION markupp_member_vaults() FROM markupp_reader;
REVOKE ALL ON note_edges, note_link_items, note_unresolved_link_items FROM markupp_reader;
REVOKE ALL ON notes, note_search_index, note_titles, note_links, note_centrality,
    note_communities, vault_graph_versions FROM markupp_reader;
REVOKE USAGE ON SCHEMA public FROM markupp_reader;
DROP TABLE IF EXISTS vault_publications;
DROP FUNCTION IF EXISTS markupp_public_vaults();
