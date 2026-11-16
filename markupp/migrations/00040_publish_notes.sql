-- +goose Up
-- Publicação de notas, pastas e cofres para leitura sem conta (ADR-0048). A publicação é
-- a linha: existir é estar publicado. O tenant vem do DEFAULT, lido da sessão, e a chave
-- com o cofre impede publicar cofre de outro tenant. A pasta é um prefixo de caminho, sem
-- barra no começo nem no fim, e cobre também as notas que entrarem nela depois.
CREATE TABLE publications (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL DEFAULT current_setting('markupp.tenant_id', true),
    vault_id     TEXT NOT NULL,
    kind         TEXT NOT NULL CHECK (kind IN ('note', 'folder', 'vault')),
    note_id      TEXT REFERENCES notes (id) ON DELETE CASCADE,
    folder_path  TEXT CHECK (folder_path ~ '^[^/]+(/[^/]+)*$'),
    indexable    BOOLEAN NOT NULL DEFAULT false,
    published_by TEXT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT publications_vault_fkey FOREIGN KEY (tenant_id, vault_id)
        REFERENCES vaults (tenant_id, id) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT publications_note_target CHECK ((kind = 'note') = (note_id IS NOT NULL)),
    CONSTRAINT publications_folder_target CHECK ((kind = 'folder') = (folder_path IS NOT NULL))
);
CREATE UNIQUE INDEX publications_vault_key ON publications (vault_id) WHERE kind = 'vault';
CREATE UNIQUE INDEX publications_folder_key ON publications (vault_id, folder_path) WHERE kind = 'folder';
CREATE UNIQUE INDEX publications_note_key ON publications (note_id) WHERE kind = 'note';
CREATE INDEX publications_vault_idx ON publications (vault_id);

-- Nota pública: o id público dela e a publicação mais ampla que a cobre. Só os gatilhos
-- escrevem aqui. Nota coberta tem linha e guarda o id enquanto alguma publicação a cobrir;
-- nota não coberta não tem linha, e publicar de novo gera outro id.
CREATE TABLE public_notes (
    note_id        TEXT PRIMARY KEY REFERENCES notes (id) ON DELETE CASCADE,
    public_id      TEXT NOT NULL UNIQUE CHECK (public_id ~ '^[A-Za-z0-9]{12}$'),
    vault_id       TEXT NOT NULL,
    publication_id TEXT NOT NULL REFERENCES publications (id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX public_notes_publication_idx ON public_notes (publication_id);

-- Id público de 12 caracteres base62. gen_random_uuid lê a fonte criptográfica do
-- servidor; os bytes de versão e variante do UUID ficam de fora, e a rejeição dos bytes
-- acima de 247 tira o viés do módulo.
-- +goose StatementBegin
CREATE FUNCTION markupp_new_public_id() RETURNS TEXT
LANGUAGE plpgsql VOLATILE SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    alphabet CONSTANT TEXT := '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
    result TEXT := '';
    bytes BYTEA;
    b INT;
BEGIN
    WHILE length(result) < 12 LOOP
        bytes := uuid_send(gen_random_uuid());
        FOR i IN 0..15 LOOP
            CONTINUE WHEN i IN (6, 8);
            b := get_byte(bytes, i);
            CONTINUE WHEN b > 247;
            result := result || substr(alphabet, b % 62 + 1, 1);
            EXIT WHEN length(result) = 12;
        END LOOP;
    END LOOP;
    RETURN result;
END;
$$;
-- +goose StatementEnd

-- A publicação mais ampla que cobre a nota: cofre, depois a pasta de caminho mais curto,
-- depois a da própria nota. p_skip é a publicação que está sendo apagada.
-- +goose StatementBegin
CREATE FUNCTION markupp_covering_publication(p_vault_id TEXT, p_note_id TEXT, p_path TEXT, p_skip TEXT)
RETURNS TEXT
LANGUAGE sql STABLE SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT p.id FROM publications p
    WHERE p.vault_id = p_vault_id AND p.id IS DISTINCT FROM p_skip
      AND (p.kind = 'vault'
           OR (p.kind = 'folder' AND starts_with(p_path, p.folder_path || '/'))
           OR (p.kind = 'note' AND p.note_id = p_note_id))
    ORDER BY CASE p.kind WHEN 'vault' THEN 0 WHEN 'folder' THEN 1 ELSE 2 END,
             length(p.folder_path), p.id
    LIMIT 1
$$;
-- +goose StatementEnd

-- Acerta a linha pública da nota pela cobertura atual: cria com id novo, troca a
-- publicação mantendo o id, ou apaga.
-- +goose StatementBegin
CREATE FUNCTION markupp_sync_public_note(p_vault_id TEXT, p_note_id TEXT, p_path TEXT, p_skip TEXT)
RETURNS void
LANGUAGE plpgsql SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    v_publication TEXT := markupp_covering_publication(p_vault_id, p_note_id, p_path, p_skip);
BEGIN
    IF v_publication IS NULL THEN
        DELETE FROM public_notes WHERE note_id = p_note_id;
        RETURN;
    END IF;
    INSERT INTO public_notes (note_id, public_id, vault_id, publication_id)
    VALUES (p_note_id, markupp_new_public_id(), p_vault_id, v_publication)
    ON CONFLICT (note_id) DO UPDATE SET publication_id = EXCLUDED.publication_id;
END;
$$;
-- +goose StatementEnd

-- Acerta as notas que a publicação alcança, na criação (p_skip nulo) e antes de apagá-la.
-- +goose StatementBegin
CREATE FUNCTION markupp_sync_publication_scope(p publications, p_skip TEXT) RETURNS void
LANGUAGE plpgsql SET search_path = pg_catalog, public, pg_temp AS $$
DECLARE
    n RECORD;
BEGIN
    FOR n IN SELECT id, path FROM notes
             WHERE vault_id = p.vault_id
               AND (p.kind = 'vault'
                    OR (p.kind = 'folder' AND starts_with(path, p.folder_path || '/'))
                    OR (p.kind = 'note' AND id = p.note_id))
    LOOP
        PERFORM markupp_sync_public_note(p.vault_id, n.id, n.path, p_skip);
    END LOOP;
END;
$$;
-- +goose StatementEnd

-- Os gatilhos rodam como o dono das tabelas: a aplicação não escreve em public_notes e
-- não enxerga as publicações dos cofres de que não é membro.
-- +goose StatementBegin
CREATE FUNCTION markupp_publication_changed() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM markupp_sync_publication_scope(NEW, NULL);
        RETURN NEW;
    END IF;
    PERFORM markupp_sync_publication_scope(OLD, OLD.id);
    RETURN OLD;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER publications_cover_notes AFTER INSERT ON publications
    FOR EACH ROW EXECUTE FUNCTION markupp_publication_changed();
CREATE TRIGGER publications_uncover_notes BEFORE DELETE ON publications
    FOR EACH ROW EXECUTE FUNCTION markupp_publication_changed();

-- Nota criada ou movida segue a cobertura do caminho novo. Nota apagada sai pela chave.
-- +goose StatementBegin
CREATE FUNCTION markupp_note_coverage_changed() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
BEGIN
    PERFORM markupp_sync_public_note(NEW.vault_id, NEW.id, NEW.path, NULL);
    RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER notes_cover_public AFTER INSERT ON notes
    FOR EACH ROW EXECUTE FUNCTION markupp_note_coverage_changed();
CREATE TRIGGER notes_recover_public AFTER UPDATE OF path ON notes
    FOR EACH ROW WHEN (OLD.path IS DISTINCT FROM NEW.path)
    EXECUTE FUNCTION markupp_note_coverage_changed();

REVOKE ALL ON FUNCTION markupp_new_public_id() FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_covering_publication(TEXT, TEXT, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_sync_public_note(TEXT, TEXT, TEXT, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_sync_publication_scope(publications, TEXT) FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_publication_changed() FROM PUBLIC;
REVOKE ALL ON FUNCTION markupp_note_coverage_changed() FROM PUBLIC;

-- Quem enxerga a publicação: owner e admin, todas do tenant; os demais, as dos cofres de
-- que são membros.
-- +goose StatementBegin
CREATE FUNCTION markupp_sees_publication(p_tenant_id TEXT, p_vault_id TEXT) RETURNS BOOLEAN
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT p_tenant_id = current_setting('markupp.tenant_id', true)
       AND (current_setting('markupp.role', true) IN ('owner', 'admin')
            OR p_vault_id IN (SELECT markupp_member_vaults()))
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_sees_publication(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_sees_publication(TEXT, TEXT) TO markupp_app;

-- Publicar a nota exige poder editá-la; pasta e cofre, owner ou admin membro do cofre.
-- Despublicar e mudar a indexação: owner e admin em tudo do tenant, os demais só a
-- publicação da nota que podem editar.
GRANT SELECT, INSERT, DELETE ON publications TO markupp_app;
GRANT UPDATE (indexable) ON publications TO markupp_app;
ALTER TABLE publications ENABLE ROW LEVEL SECURITY;
CREATE POLICY publications_read ON publications FOR SELECT TO markupp_app
    USING (markupp_sees_publication(tenant_id, vault_id));
CREATE POLICY publications_insert ON publications FOR INSERT TO markupp_app
    WITH CHECK (tenant_id = current_setting('markupp.tenant_id', true)
                AND published_by = current_setting('markupp.user_id', true)
                AND vault_id IN (SELECT markupp_member_vaults())
                AND ((kind = 'note' AND current_setting('markupp.role', true) <> 'viewer'
                      AND EXISTS (SELECT 1 FROM notes n WHERE n.id = publications.note_id AND n.vault_id = publications.vault_id))
                     OR (kind IN ('folder', 'vault') AND current_setting('markupp.role', true) IN ('owner', 'admin'))));
CREATE POLICY publications_update ON publications FOR UPDATE TO markupp_app
    USING (markupp_sees_publication(tenant_id, vault_id)
           AND (current_setting('markupp.role', true) IN ('owner', 'admin')
                OR (kind = 'note' AND current_setting('markupp.role', true) <> 'viewer'
                    AND vault_id IN (SELECT markupp_member_vaults()))));
CREATE POLICY publications_delete ON publications FOR DELETE TO markupp_app
    USING (markupp_sees_publication(tenant_id, vault_id)
           AND (current_setting('markupp.role', true) IN ('owner', 'admin')
                OR (kind = 'note' AND current_setting('markupp.role', true) <> 'viewer'
                    AND vault_id IN (SELECT markupp_member_vaults()))));

-- O id público das notas que o membro lê, para o painel marcar a nota pública.
GRANT SELECT ON public_notes TO markupp_app;
ALTER TABLE public_notes ENABLE ROW LEVEL SECURITY;
CREATE POLICY public_notes_read ON public_notes FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- As publicações que o ator enxerga, com os nomes que o RLS das notas e dos cofres
-- esconderia do owner e do admin fora do cofre. Quem publicou só tem email quando ainda
-- é do tenant, como na política de users. p_id vazio lista todas.
-- +goose StatementBegin
CREATE FUNCTION markupp_list_publications(p_vault_id TEXT, p_id TEXT)
RETURNS TABLE (id TEXT, vault_id TEXT, vault_name TEXT, kind TEXT, note_id TEXT, note_path TEXT,
               folder_path TEXT, note_count BIGINT, indexable BOOLEAN, public_id TEXT,
               published_by TEXT, published_by_email TEXT, published_at TIMESTAMPTZ)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT p.id, p.vault_id, v.name, p.kind, p.note_id, n.path, p.folder_path,
           (SELECT count(*) FROM notes c
            WHERE c.vault_id = p.vault_id
              AND (p.kind = 'vault'
                   OR (p.kind = 'folder' AND starts_with(c.path, p.folder_path || '/'))
                   OR (p.kind = 'note' AND c.id = p.note_id))),
           p.indexable, pn.public_id, p.published_by, u.email, p.published_at
    FROM publications p
    JOIN vaults v ON v.id = p.vault_id
    LEFT JOIN notes n ON n.id = p.note_id
    LEFT JOIN public_notes pn ON pn.note_id = p.note_id
    LEFT JOIN users u ON u.id = p.published_by
         AND u.id IN (SELECT m.user_id FROM tenant_memberships m WHERE m.tenant_id = p.tenant_id)
    WHERE markupp_sees_publication(p.tenant_id, p.vault_id)
      AND (p_vault_id = '' OR p.vault_id = p_vault_id)
      AND (p_id = '' OR p.id = p_id)
    ORDER BY p.published_at DESC, p.id
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_list_publications(TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_list_publications(TEXT, TEXT) TO markupp_app;

-- Papel da leitura sem conta: só lê, e só as notas públicas. A API troca para ele com
-- SET LOCAL ROLE numa transação própria, que zera tenant, usuário e papel da sessão.
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

-- Notas públicas. SECURITY DEFINER para as políticas do leitor não dependerem do grant
-- dele em public_notes.
-- +goose StatementBegin
CREATE FUNCTION markupp_public_note_ids() RETURNS SETOF TEXT
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT note_id FROM public_notes
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_public_note_ids() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_public_note_ids() TO markupp_reader;

-- A publicação da nota pública, sem quem publicou nem o tenant. O nome do cofre só sai
-- quando o cofre inteiro está publicado. O leitor não tem grant em publications.
-- +goose StatementBegin
CREATE FUNCTION markupp_public_scope(p_public_id TEXT)
RETURNS TABLE (note_id TEXT, vault_id TEXT, publication_id TEXT, kind TEXT, folder_path TEXT,
               indexable BOOLEAN, vault_name TEXT)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public, pg_temp AS $$
    SELECT pn.note_id, pn.vault_id, p.id, p.kind, p.folder_path, p.indexable,
           CASE WHEN p.kind = 'vault' THEN v.name END
    FROM public_notes pn
    JOIN publications p ON p.id = pn.publication_id
    JOIN vaults v ON v.id = p.vault_id
    WHERE pn.public_id = p_public_id
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION markupp_public_scope(TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION markupp_public_scope(TEXT) TO markupp_reader;

-- O leitor só recebe SELECT, e só nas tabelas e views da nota e da organização derivada
-- que as rotas públicas leem, recortadas às notas públicas.
GRANT USAGE ON SCHEMA public TO markupp_reader;
GRANT SELECT ON notes, public_notes, note_search_index, note_titles, note_links,
    note_centrality, note_communities, vault_graph_versions TO markupp_reader;
GRANT SELECT ON note_edges, note_link_items, note_unresolved_link_items TO markupp_reader;
-- As políticas de notes sem papel (00007) valem para todos e chamam esta função, que
-- para o leitor, sem usuário na sessão, não devolve nada.
GRANT EXECUTE ON FUNCTION markupp_member_vaults() TO markupp_reader;

CREATE POLICY public_notes_public ON public_notes FOR SELECT TO markupp_reader
    USING (true);
CREATE POLICY notes_public ON notes FOR SELECT TO markupp_reader
    USING (id IN (SELECT markupp_public_note_ids()));
CREATE POLICY note_search_index_public ON note_search_index FOR SELECT TO markupp_reader
    USING (note_id IN (SELECT markupp_public_note_ids()));
CREATE POLICY note_titles_public ON note_titles FOR SELECT TO markupp_reader
    USING (note_id IN (SELECT markupp_public_note_ids()));
CREATE POLICY note_links_public ON note_links FOR SELECT TO markupp_reader
    USING (note_id IN (SELECT markupp_public_note_ids()));
CREATE POLICY note_centrality_public ON note_centrality FOR SELECT TO markupp_reader
    USING (note_id IN (SELECT markupp_public_note_ids()));
CREATE POLICY note_communities_public ON note_communities FOR SELECT TO markupp_reader
    USING (note_id IN (SELECT markupp_public_note_ids()));
CREATE POLICY vault_graph_versions_public ON vault_graph_versions FOR SELECT TO markupp_reader
    USING (vault_id IN (SELECT pn.vault_id FROM public_notes pn));

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
DROP TRIGGER IF EXISTS notes_recover_public ON notes;
DROP TRIGGER IF EXISTS notes_cover_public ON notes;
DROP FUNCTION IF EXISTS markupp_sync_publication_scope(publications, TEXT);
DROP TABLE IF EXISTS public_notes;
DROP TABLE IF EXISTS publications;
DROP FUNCTION IF EXISTS markupp_public_scope(TEXT);
DROP FUNCTION IF EXISTS markupp_public_note_ids();
DROP FUNCTION IF EXISTS markupp_list_publications(TEXT, TEXT);
DROP FUNCTION IF EXISTS markupp_sees_publication(TEXT, TEXT);
DROP FUNCTION IF EXISTS markupp_note_coverage_changed();
DROP FUNCTION IF EXISTS markupp_publication_changed();
DROP FUNCTION IF EXISTS markupp_sync_public_note(TEXT, TEXT, TEXT, TEXT);
DROP FUNCTION IF EXISTS markupp_covering_publication(TEXT, TEXT, TEXT, TEXT);
DROP FUNCTION IF EXISTS markupp_new_public_id();
