-- +goose Up
-- create "catalogo_arquivos" table
CREATE TABLE "catalogo_arquivos" ("caminho" character varying NOT NULL, "conteudo" bytea NOT NULL, "sha256" character varying NOT NULL, "atualizado_em" timestamptz NOT NULL, PRIMARY KEY ("caminho"), CONSTRAINT "catalogo_arquivos_caminho_na_raiz" CHECK (((caminho)::text ~~ 'agentes/%'::text) OR ((caminho)::text ~~ 'nexus/%'::text)));

-- +goose Down
-- reverse: create "catalogo_arquivos" table
DROP TABLE "catalogo_arquivos";
