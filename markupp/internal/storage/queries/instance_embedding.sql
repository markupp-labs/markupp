-- name: GetInstanceEmbedding :one
SELECT provider, url, model, api_key_sealed, updated_at,
       host_locked, host_provider, model_state, model_error, model_state_at
FROM instance_embedding;

-- clock_timestamp, e não now, para duas gravações na mesma transação mudarem a revisão.
-- name: SaveInstanceEmbeddingChoice :execrows
UPDATE instance_embedding
SET provider = $1, url = $2, model = $3, api_key_sealed = $4, updated_at = clock_timestamp();

-- Só escreve quando muda, porque o indexador informa a cada volta do laço.
-- name: ReportInstanceEmbeddingHost :exec
UPDATE instance_embedding SET host_locked = $1, host_provider = $2
WHERE host_locked IS DISTINCT FROM $1 OR host_provider IS DISTINCT FROM $2;

-- name: ReportInstanceEmbeddingModel :exec
UPDATE instance_embedding SET model_state = $1, model_error = $2, model_state_at = clock_timestamp();

-- Novo selo da chave com a chave de cifra ativa: só troca o selo lido, e não muda a
-- revisão, para o indexador de embedding não reabrir o provedor.
-- name: ResealInstanceEmbeddingKey :exec
UPDATE instance_embedding SET api_key_sealed = sqlc.arg(fresh) WHERE api_key_sealed = sqlc.arg(old);
