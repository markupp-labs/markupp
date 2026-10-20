-- +goose Up
-- Filtro de keyphraseness (Mihalcea e Csomai, 2007): o título cujo texto aparece em mais
-- de max(5% das notas do cofre, 5) notas é palavra comum, e não entidade. Ele não vira
-- menção não ligada nem entra na probabilidade a priori da ligação inferida. O piso de
-- cinco notas evita que o cofre pequeno, em que 5% não chega a uma nota, perca as menções. A leitura da menção
-- passa a consultar o df e o N do cofre, então a aplicação os lê pelos cofres do ator.
GRANT SELECT ON vault_term_frequencies, vault_term_stats TO markupp_app;
CREATE POLICY vault_term_frequencies_read ON vault_term_frequencies FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));
CREATE POLICY vault_term_stats_read ON vault_term_stats FOR SELECT TO markupp_app
    USING (vault_id IN (SELECT markupp_member_vaults()));

-- Notas do cofre com o título no corpo (peso D), como a menção conta as ocorrências
-- da âncora, ou NULL quando o título é comum. O teto é o maior entre 5% do N gravado
-- pelo segundo estágio e 5. Título de um lexema é julgado antes pelo df gravado, em
-- O(1): o df conta o documento inteiro, título incluído, então desconta as notas que
-- têm esse título. A
-- palavra no título de outras notas (o título "projeto alfa" para "projeto") continua
-- no df, e conta como presença na nota. Os outros títulos, e os de um lexema
-- que passam, pagam a contagem no GIN, que para no teto. Cofre sem N ainda não tem
-- como julgar e conta tudo, sem filtro. Roda com os direitos de quem chama.
-- +goose StatementBegin
CREATE FUNCTION markupp_title_occurrences(p_vault_id TEXT, p_config REGCONFIG, p_title_key TEXT) RETURNS INTEGER
LANGUAGE plpgsql STABLE AS $$
DECLARE
    v_query       TSQUERY := phraseto_tsquery(p_config, p_title_key);
    v_ceiling     INTEGER := (SELECT greatest(floor(k.note_count * 0.05)::int, 5) FROM vault_term_stats k
                              WHERE k.vault_id = p_vault_id AND k.note_count > 0);
    v_body_floor  INTEGER;
    v_occurrences INTEGER;
BEGIN
    IF v_ceiling IS NOT NULL AND numnode(v_query) = 1 THEN
        SELECT f.df - (SELECT count(*) FROM note_titles t WHERE t.vault_id = p_vault_id AND t.title_key = p_title_key)
        INTO v_body_floor
        FROM vault_term_frequencies f
        WHERE f.vault_id = p_vault_id AND f.term = (tsvector_to_array(to_tsvector(p_config, p_title_key)))[1];
        IF v_body_floor > v_ceiling THEN
            RETURN NULL;
        END IF;
    END IF;
    SELECT count(*) INTO v_occurrences FROM (
        SELECT 1 FROM note_search_index s
        JOIN notes n ON n.id = s.note_id
        WHERE n.vault_id = p_vault_id AND s.document @@ v_query AND ts_filter(s.document, '{d}') @@ v_query
        LIMIT v_ceiling + 1
    ) found;
    IF v_occurrences > v_ceiling THEN
        RETURN NULL;
    END IF;
    RETURN v_occurrences;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS markupp_title_occurrences(TEXT, REGCONFIG, TEXT);
DROP POLICY IF EXISTS vault_term_stats_read ON vault_term_stats;
DROP POLICY IF EXISTS vault_term_frequencies_read ON vault_term_frequencies;
REVOKE SELECT ON vault_term_frequencies, vault_term_stats FROM markupp_app;
