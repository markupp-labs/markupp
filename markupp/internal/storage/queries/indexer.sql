-- O indexador reserva o cofre no papel da conexão, que enxerga todos os tenants, e
-- só então entra no papel markupp_indexer, preso pelo RLS ao cofre reservado.

-- Trava a linha do cofre, e não a fila, para dois Jobs nunca pegarem o mesmo cofre. NO KEY
-- UPDATE não conflita com a checagem de chave estrangeira, então as notas seguem
-- graváveis. A ordem aleatória impede que um cofre que sempre falha segure os outros. O
-- cofre só marcado para o segundo estágio também é pendente, desde que nenhum Job já
-- esteja no segundo estágio dele.
-- name: ReservePendingVault :one
SELECT v.id AS vault_id, v.tenant_id
FROM vaults v
WHERE v.id IN (SELECT n.vault_id FROM note_index_queue q JOIN notes n ON n.id = q.note_id)
   OR v.id IN (SELECT s.vault_id FROM vault_second_stage_queue s WHERE NOT markupp_second_stage_running(s.vault_id))
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

-- O lote substitui as linhas do grafo das notas dele: apaga e grava de novo. A
-- ligação inferida é do segundo estágio, que a refaz quando o grafo muda.
-- name: DeleteNoteGraph :exec
WITH apagadas AS (DELETE FROM note_links WHERE note_id = ANY (sqlc.arg(note_ids)::text[]) AND kind <> 'inferred')
DELETE FROM note_titles WHERE note_id = ANY (sqlc.arg(note_ids)::text[]);

-- O cofre vem da nota. Nota apagada depois da leitura some no JOIN.
-- name: SaveNoteTitles :exec
INSERT INTO note_titles (note_id, vault_id, title_key, path)
SELECT n.id, n.vault_id, d.title_key, n.path
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(title_keys)::text[]) AS title_key) d
JOIN notes n ON n.id = d.note_id;

-- Caminho vazio é ligação sem caminho, isto é, que não veio de link markdown, e
-- predicado vazio é ligação que não é tipada.
-- name: SaveNoteLinks :exec
INSERT INTO note_links (note_id, vault_id, cited_title, title_key, target_path, kind, predicate)
SELECT n.id, n.vault_id, d.cited_title, d.title_key, NULLIF(d.target_path, ''), d.kind, NULLIF(d.predicate, '')
FROM (SELECT unnest(sqlc.arg(note_ids)::text[]) AS note_id, unnest(sqlc.arg(cited_titles)::text[]) AS cited_title,
             unnest(sqlc.arg(title_keys)::text[]) AS title_key, unnest(sqlc.arg(target_paths)::text[]) AS target_path,
             unnest(sqlc.arg(kinds)::text[]) AS kind, unnest(sqlc.arg(predicates)::text[]) AS predicate) d
JOIN notes n ON n.id = d.note_id;

-- Impressão dos títulos, dos caminhos e das ligações das notas do lote, para saber se o
-- lote os mudou. A ligação inferida fica de fora porque o lote não a grava.
-- name: NoteGraphFingerprint :one
SELECT md5(
    coalesce((SELECT string_agg(ROW(t.note_id, t.title_key, t.path)::text, ',' ORDER BY ROW(t.note_id, t.title_key, t.path)::text)
              FROM note_titles t WHERE t.note_id = ANY (sqlc.arg(note_ids)::text[])), '')
    || ';' ||
    coalesce((SELECT string_agg(ROW(l.note_id, l.title_key, l.target_path, l.kind, l.predicate)::text, ','
                                ORDER BY ROW(l.note_id, l.title_key, l.target_path, l.kind, l.predicate)::text)
              FROM note_links l WHERE l.note_id = ANY (sqlc.arg(note_ids)::text[]) AND l.kind <> 'inferred'), '')
)::text AS fingerprint;

-- name: MarkSecondStage :exec
INSERT INTO vault_second_stage_queue (vault_id) VALUES (sqlc.arg(vault_id))
ON CONFLICT (vault_id) DO UPDATE SET revision = nextval('vault_second_stage_revision_seq');

-- Falso quando outro Job já está no segundo estágio do cofre. Solta no fim da transação.
-- name: TryLockSecondStage :one
SELECT markupp_try_lock_second_stage(sqlc.arg(vault_id)::text)::bool AS locked;

-- Notas do cofre no último df, para decidir se o lote foi grande frente a ele sem
-- contar as notas do cofre a cada lote. Cofre ainda sem df conta zero.
-- name: VaultTermNoteCount :one
SELECT coalesce((SELECT k.note_count FROM vault_term_stats k WHERE k.vault_id = sqlc.arg(vault_id)), 0)::int AS note_count;

-- name: SecondStageRevision :one
SELECT revision FROM vault_second_stage_queue WHERE vault_id = sqlc.arg(vault_id);

-- Desmarca só a revisão lida: a mudança confirmada depois dela deixa o cofre marcado.
-- name: UnmarkSecondStage :exec
DELETE FROM vault_second_stage_queue WHERE vault_id = sqlc.arg(vault_id) AND revision = sqlc.arg(revision);

-- name: DeleteVaultTermFrequencies :exec
DELETE FROM vault_term_frequencies WHERE vault_id = sqlc.arg(vault_id);

-- df de cada lexema do índice léxico das notas do cofre.
-- name: SaveVaultTermFrequencies :exec
INSERT INTO vault_term_frequencies (vault_id, term, df)
SELECT n.vault_id, u.lexeme, count(*)
FROM notes n
JOIN note_search_index s ON s.note_id = n.id
CROSS JOIN LATERAL unnest(s.document) AS u (lexeme, positions, weights)
WHERE n.vault_id = sqlc.arg(vault_id)
GROUP BY n.vault_id, u.lexeme;

-- name: DeleteNoteTerms :exec
DELETE FROM note_terms WHERE note_id = ANY (sqlc.arg(note_ids)::text[]);

-- Os termos de maior tf-idf de cada nota do lote, tirados do documento que o lote
-- acabou de gravar, com o df e o N vigentes, gravados com o tf, o peso e a norma do
-- vetor. tf = 1 + log10 das posições do lexema. A ordem por nota, no LATERAL, ordena
-- poucos lexemas de cada vez em memória. Documento sem lexema não grava linha. O cofre
-- vem do parâmetro, e não de uma junção com notes: no papel do indexador o RLS de
-- notes estima poucas linhas, e o plano percorria as notas do cofre sondando o lote a
-- cada uma, O(N x B). A política de note_search_index já descarta nota apagada.
-- name: SaveNoteTerms :exec
WITH stats AS (
    SELECT coalesce((SELECT k.note_count FROM vault_term_stats k WHERE k.vault_id = sqlc.arg(vault_id)), 0) AS note_count
)
INSERT INTO note_terms (note_id, vault_id, terms, tfs, weights, norm)
SELECT s.note_id, sqlc.arg(vault_id), top.terms, top.tfs, top.weights, top.norm
FROM note_search_index s
CROSS JOIN stats
CROSS JOIN LATERAL (
    SELECT array_agg(w.term ORDER BY w.weight DESC, w.term)::text[] AS terms,
           array_agg(w.tf ORDER BY w.weight DESC, w.term)::float8[] AS tfs,
           array_agg(w.weight ORDER BY w.weight DESC, w.term)::float8[] AS weights,
           sqrt(sum(w.weight ^ 2))::float8 AS norm
    FROM (
        SELECT u.lexeme AS term, 1 + log(cardinality(u.positions)::float8) AS tf,
               (1 + log(cardinality(u.positions)::float8))
               * markupp_term_idf(stats.note_count,
                    (SELECT f.df FROM vault_term_frequencies f WHERE f.vault_id = sqlc.arg(vault_id) AND f.term = u.lexeme),
                    stats.note_count > 0) AS weight
        FROM unnest(s.document) AS u (lexeme, positions, weights)
        ORDER BY weight DESC, term
        LIMIT sqlc.arg(terms_per_note)::int
    ) w
) top
WHERE s.note_id = ANY (sqlc.arg(note_ids)::text[]) AND top.terms IS NOT NULL;

-- Se o cofre já tem df, pelo mesmo critério com que SaveNoteTerms usa o idf.
-- name: VaultTermStatsKnown :one
SELECT coalesce((SELECT k.note_count > 0 FROM vault_term_stats k WHERE k.vault_id = sqlc.arg(vault_id)), false)::bool AS known;

-- Põe as notas do cofre de volta na fila, com revisão nova para o lote que já as leu
-- deixá-las pendentes.
-- name: RequeueVaultNotes :exec
INSERT INTO note_index_queue (note_id)
SELECT id FROM notes WHERE vault_id = sqlc.arg(vault_id)
ON CONFLICT (note_id) DO UPDATE SET revision = nextval('note_revision_seq');

-- N do idf: as notas do cofre com documento, as mesmas que o df conta.
-- name: SaveVaultTermStats :exec
INSERT INTO vault_term_stats (vault_id, note_count)
SELECT sqlc.arg(vault_id), count(*)
FROM notes n JOIN note_search_index s ON s.note_id = n.id
WHERE n.vault_id = sqlc.arg(vault_id)
ON CONFLICT (vault_id) DO UPDATE SET note_count = EXCLUDED.note_count;

-- Repesa as postagens do cofre com o df e o N recém-gravados, na ordem dos termos, numa
-- junção só entre os termos de todas as notas e o df do cofre. A linha que um lote
-- concorrente está regravando fica de fora: o lote a grava com o df anterior, e o
-- segundo estágio não espera por ele nem o faz esperar além do commit.
-- name: ReweightVaultNoteTerms :exec
UPDATE note_terms t
SET weights = r.weights, norm = r.norm
FROM (
    SELECT w.note_id, array_agg(w.weight ORDER BY w.ord)::float8[] AS weights, sqrt(sum(w.weight ^ 2))::float8 AS norm
    FROM (
        SELECT p.note_id, p.ord, p.tf * markupp_term_idf(st.note_count, f.df, st.note_count > 0) AS weight
        FROM (
            SELECT o.note_id, i.ord, o.terms[i.ord] AS term, o.tfs[i.ord] AS tf
            FROM (SELECT l.note_id, l.terms, l.tfs FROM note_terms l WHERE l.vault_id = sqlc.arg(vault_id) FOR UPDATE SKIP LOCKED) o
            CROSS JOIN LATERAL generate_subscripts(o.terms, 1) AS i (ord)
        ) p
        CROSS JOIN (SELECT k.note_count FROM vault_term_stats k WHERE k.vault_id = sqlc.arg(vault_id)) st
        LEFT JOIN vault_term_frequencies f ON f.vault_id = sqlc.arg(vault_id) AND f.term = p.term
    ) w
    GROUP BY w.note_id
) r
WHERE t.note_id = r.note_id;
