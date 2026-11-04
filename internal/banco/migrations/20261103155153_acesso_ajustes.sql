-- +goose Up
-- drop index "papeis_herda_de_key" from table: "papeis"
DROP INDEX "papeis_herda_de_key";
-- modify "papeis" table
ALTER TABLE "papeis" DROP CONSTRAINT "papeis_papeis_herda", ADD CONSTRAINT "papeis_papeis_filhos" FOREIGN KEY ("herda_de") REFERENCES "papeis" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT;
-- modify "atribuicoes" table
ALTER TABLE "atribuicoes" ADD CONSTRAINT "atribuicoes_administradora_id" CHECK (((escopo_tipo)::text = 'condominio'::text) = (administradora_id IS NOT NULL)), ADD COLUMN "administradora_id" character varying NULL, ADD CONSTRAINT "atribuicoes_administradoras_administradora" FOREIGN KEY ("administradora_id") REFERENCES "administradoras" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT;

-- +goose Down
-- reverse: modify "atribuicoes" table
ALTER TABLE "atribuicoes" DROP CONSTRAINT "atribuicoes_administradoras_administradora", DROP COLUMN "administradora_id", DROP CONSTRAINT "atribuicoes_administradora_id";
-- reverse: modify "papeis" table
ALTER TABLE "papeis" DROP CONSTRAINT "papeis_papeis_filhos", ADD CONSTRAINT "papeis_papeis_herda" FOREIGN KEY ("herda_de") REFERENCES "papeis" ("id") ON UPDATE NO ACTION ON DELETE RESTRICT;
-- reverse: drop index "papeis_herda_de_key" from table: "papeis"
CREATE UNIQUE INDEX "papeis_herda_de_key" ON "papeis" ("herda_de");
