-- +goose Up
-- Grafo de ligações no plano de recuperação (ADR-0012). O indexador grava só o que cada
-- nota declara: os títulos dela e os títulos que ela cita. O destino é resolvido na
-- leitura, por junção de título, então renomear ou apagar uma nota não deixa linha de
-- outra nota velha. O cofre vem de notes na gravação, e a nota nunca muda de cofre. O
-- tenant não fica na linha porque o cofre muda de tenant na adoção de cofres legados:
-- ele vem do cofre, pelas políticas.

-- Chaves de título da nota (caminho e aliases), já normalizadas pelo indexador.
CREATE TABLE note_titles (
    note_id   TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id  TEXT NOT NULL,
    title_key TEXT NOT NULL,
    PRIMARY KEY (note_id, title_key)
);
CREATE INDEX note_titles_vault_title_idx ON note_titles (vault_id, title_key);

-- Ligações que a nota faz: o título citado como escrito e a chave dele, e o caminho
-- quando é link markdown. Os tipos typed e inferred chegam com o frontmatter e com o
-- segundo estágio.
CREATE TABLE note_links (
    note_id     TEXT NOT NULL REFERENCES notes (id) ON DELETE CASCADE ON UPDATE CASCADE,
    vault_id    TEXT NOT NULL,
    cited_title TEXT NOT NULL,
    title_key   TEXT NOT NULL,
    target_path TEXT,
    kind        TEXT NOT NULL CHECK (kind IN ('explicit', 'embed', 'typed', 'inferred'))
);
CREATE INDEX note_links_note_idx ON note_links (note_id);
CREATE INDEX note_links_vault_title_idx ON note_links (vault_id, title_key);

-- Ligação resolvida: liga a todas as notas com o título citado, salvo o link markdown
-- cujo caminho existe, que liga só à nota daquele caminho. A nota que cita a si mesma
-- não ganha aresta. security_invoker faz a leitura passar pelo RLS de quem consulta.
CREATE VIEW note_edges WITH (security_invoker = true) AS
SELECT l.note_id AS source_id, t.note_id AS target_id, l.vault_id, l.kind
FROM note_links l
JOIN note_titles t ON t.vault_id = l.vault_id AND t.title_key = l.title_key AND t.note_id <> l.note_id
WHERE l.target_path IS NULL
   OR EXISTS (SELECT 1 FROM notes p WHERE p.id = t.note_id AND p.path = l.target_path)
   OR NOT EXISTS (SELECT 1 FROM notes p WHERE p.vault_id = l.vault_id AND p.path = l.target_path);

-- O indexador passa pela política notes_indexer, presa ao tenant e ao cofre da sessão; a
-- aplicação, por markupp_member_vaults, que só devolve cofres do tenant da sessão.
GRANT SELECT, INSERT, DELETE ON note_titles, note_links TO markupp_indexer;
GRANT SELECT ON note_titles, note_links, note_edges TO markupp_app;

ALTER TABLE note_titles ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_titles_indexer ON note_titles TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_titles_read ON note_titles FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

ALTER TABLE note_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY note_links_indexer ON note_links TO markupp_indexer
    USING (vault_id = current_setting('markupp.vault_id', true)
           AND EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));
CREATE POLICY note_links_read ON note_links FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- As notas já indexadas não têm ligações: todas voltam para a fila.
SELECT markupp_requeue_notes(NULL);

-- +goose Down
DROP VIEW IF EXISTS note_edges;
DROP TABLE IF EXISTS note_links;
DROP TABLE IF EXISTS note_titles;
