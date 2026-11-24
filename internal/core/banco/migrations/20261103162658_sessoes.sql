-- +goose Up
-- create "sessoes" table
CREATE TABLE "sessoes" ("id" character varying NOT NULL, "administradora_id" character varying NOT NULL DEFAULT '', "condominio_id" character varying NOT NULL DEFAULT '', "protocolo" character varying NOT NULL DEFAULT '', "atualizada_em" timestamptz NOT NULL, "dados" jsonb NOT NULL, PRIMARY KEY ("id"));
-- create index "sessao_administradora_id_atualizada_em" to table: "sessoes"
CREATE INDEX "sessao_administradora_id_atualizada_em" ON "sessoes" ("administradora_id", "atualizada_em");
-- create index "sessao_atualizada_em" to table: "sessoes"
CREATE INDEX "sessao_atualizada_em" ON "sessoes" ("atualizada_em");

-- +goose Down
-- reverse: create index "sessao_atualizada_em" to table: "sessoes"
DROP INDEX "sessao_atualizada_em";
-- reverse: create index "sessao_administradora_id_atualizada_em" to table: "sessoes"
DROP INDEX "sessao_administradora_id_atualizada_em";
-- reverse: create "sessoes" table
DROP TABLE "sessoes";
