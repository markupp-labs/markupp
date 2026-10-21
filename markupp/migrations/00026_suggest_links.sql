-- +goose Up
-- Os dois sinais da sugestão de ligação, calculados na leitura para uma nota (ADR-0036).
-- As funções rodam com os direitos de quem chama, então o RLS das tabelas vale.

-- Alocação de recursos (Zhou, Lü e Zhang, 2009) na projeção não dirigida, em que toda
-- ligação conta: para cada nota a dois saltos, a soma de 1/grau de cada vizinho em
-- comum. Fora a própria nota e as já ligadas.
-- +goose StatementBegin
CREATE FUNCTION markupp_resource_allocation(p_note_id TEXT)
RETURNS TABLE (note_id TEXT, score DOUBLE PRECISION)
LANGUAGE sql STABLE AS $$
    WITH gx AS (SELECT markupp_linked_notes(p_note_id) AS id),
    gz AS (SELECT gx.id AS z, y.id AS y FROM gx CROSS JOIN LATERAL markupp_linked_notes(gx.id) AS y (id)),
    deg AS (SELECT gz.z, count(*) AS degree FROM gz GROUP BY gz.z)
    SELECT gz.y, sum(1.0 / deg.degree)::double precision
    FROM gz JOIN deg ON deg.z = gz.z
    WHERE gz.y <> p_note_id AND gz.y NOT IN (SELECT gx.id FROM gx)
    GROUP BY gz.y
$$;
-- +goose StatementEnd

-- Cosseno tf-idf entre a nota e as candidatas, sobre as postagens gravadas: pesos e
-- normas vêm prontos do indexador, e a leitura não toca no df. Candidata é a nota que
-- tem algum dos p_query_terms termos de maior peso positivo da nota, achada pelo GIN;
-- o produto escalar usa todos os termos gravados das duas. Fora a própria nota e as
-- já ligadas.
-- +goose StatementBegin
CREATE FUNCTION markupp_lexical_similarity(p_vault_id TEXT, p_note_id TEXT, p_query_terms INTEGER)
RETURNS TABLE (note_id TEXT, score DOUBLE PRECISION)
LANGUAGE sql STABLE AS $$
    WITH x AS (
        SELECT u.term, u.weight, t.norm
        FROM note_terms t CROSS JOIN LATERAL unnest(t.terms, t.weights) AS u (term, weight)
        WHERE t.vault_id = p_vault_id AND t.note_id = p_note_id AND u.weight > 0
    ),
    query AS (
        SELECT array_agg(q.term) AS terms
        FROM (SELECT x.term FROM x ORDER BY x.weight DESC, x.term LIMIT p_query_terms) q
    )
    SELECT o.note_id, d.dot / (o.norm * (SELECT max(x.norm) FROM x))
    FROM query
    JOIN note_terms o ON o.terms && query.terms
    CROSS JOIN LATERAL (
        SELECT sum(u.weight * x.weight) AS dot
        FROM unnest(o.terms, o.weights) AS u (term, weight) JOIN x ON x.term = u.term
    ) d
    WHERE o.vault_id = p_vault_id AND o.note_id <> p_note_id
      AND o.note_id NOT IN (SELECT markupp_linked_notes(p_note_id))
      AND d.dot > 0
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS markupp_lexical_similarity(TEXT, TEXT, INTEGER);
DROP FUNCTION IF EXISTS markupp_resource_allocation(TEXT);
