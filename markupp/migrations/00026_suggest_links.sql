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

-- Pesos tf-idf dos termos gravados das notas p_note_ids do cofre, com o df vigente.
-- +goose StatementBegin
CREATE FUNCTION markupp_note_term_weights(p_vault_id TEXT, p_note_ids TEXT[])
RETURNS TABLE (note_id TEXT, term TEXT, weight DOUBLE PRECISION)
LANGUAGE sql STABLE AS $$
    WITH stats AS (
        SELECT (SELECT count(*) FROM notes c WHERE c.vault_id = p_vault_id) AS note_count,
               EXISTS (SELECT 1 FROM vault_term_frequencies k WHERE k.vault_id = p_vault_id) AS df_known
    )
    SELECT t.note_id, u.term, u.tf * markupp_term_idf(stats.note_count, f.df, stats.df_known)
    FROM note_terms t
    CROSS JOIN LATERAL unnest(t.terms, t.tfs) AS u (term, tf)
    CROSS JOIN stats
    LEFT JOIN vault_term_frequencies f ON f.vault_id = t.vault_id AND f.term = u.term
    WHERE t.vault_id = p_vault_id AND t.note_id = ANY (p_note_ids)
$$;
-- +goose StatementEnd

-- Cosseno tf-idf entre a nota e as candidatas que dividem com ela algum termo de peso
-- positivo, sobre os termos gravados de cada uma. Fora a própria nota e as já ligadas.
-- +goose StatementBegin
CREATE FUNCTION markupp_lexical_similarity(p_vault_id TEXT, p_note_id TEXT)
RETURNS TABLE (note_id TEXT, score DOUBLE PRECISION)
LANGUAGE sql STABLE AS $$
    WITH x AS (SELECT w.term, w.weight FROM markupp_note_term_weights(p_vault_id, ARRAY[p_note_id]) w),
    cand AS (
        SELECT coalesce(array_agg(o.note_id), '{}') AS ids
        FROM note_terms o
        WHERE o.terms && (SELECT array_agg(x.term) FROM x WHERE x.weight > 0)
          AND o.vault_id = p_vault_id AND o.note_id <> p_note_id
          AND o.note_id NOT IN (SELECT markupp_linked_notes(p_note_id))
    ),
    scored AS (
        SELECT cw.note_id, sum(cw.weight * coalesce(x.weight, 0)) AS dot, sqrt(sum(cw.weight ^ 2)) AS norm
        FROM cand CROSS JOIN LATERAL markupp_note_term_weights(p_vault_id, cand.ids) cw
        LEFT JOIN x ON x.term = cw.term
        GROUP BY cw.note_id
    )
    SELECT s.note_id, s.dot / (s.norm * (SELECT sqrt(sum(x.weight ^ 2)) FROM x))
    FROM scored s
    WHERE s.dot > 0
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS markupp_lexical_similarity(TEXT, TEXT);
DROP FUNCTION IF EXISTS markupp_note_term_weights(TEXT, TEXT[]);
DROP FUNCTION IF EXISTS markupp_resource_allocation(TEXT);
