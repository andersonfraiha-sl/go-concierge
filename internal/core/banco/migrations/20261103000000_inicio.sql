-- Escrita à mão: o ent não declara extensões. As migrations seguintes saem do Atlas
-- (go run ./internal/banco/cmd/gerar-migration <nome>).
-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose Down
DROP EXTENSION IF EXISTS pgcrypto;
