-- +goose Up
-- A API passa a buscar no índice léxico. A linha herda o recorte de notes: a subconsulta
-- passa pela política notes_read, então quem lê a nota, o viewer inclusive, lê o
-- documento dela.
GRANT SELECT ON note_search_index TO markupp_app;
CREATE POLICY note_search_index_read ON note_search_index FOR SELECT TO markupp_app
    USING (EXISTS (SELECT 1 FROM notes n WHERE n.id = note_id));

-- +goose Down
DROP POLICY IF EXISTS note_search_index_read ON note_search_index;
REVOKE SELECT ON note_search_index FROM markupp_app;
