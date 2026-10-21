-- +goose Up
-- O documento do índice passa a levar o título com peso A antes do corpo com peso D, e
-- só o indexador o recalcula: todas as notas voltam para a fila.
SELECT markupp_requeue_notes(NULL);

-- +goose Down
-- O documento com título continua servindo à busca antiga; nada a desfazer.
