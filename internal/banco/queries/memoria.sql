-- por que não ent: apagar as chaves que saíram e fazer upsert das restantes num comando só; o ent faria uma ida por fato
-- name: GravarFatos :exec
WITH saidas AS (
    DELETE FROM memoria_fatos
    WHERE administradora_id = @administradora_id::text
      AND telefone = @telefone::text
      AND chave <> ALL(@chaves::text[])
)
INSERT INTO memoria_fatos (administradora_id, telefone, chave, condominio_id, texto, fonte, quando, confianca)
-- várias unnest na lista do SELECT andam juntas, linha a linha (o sqlc não conhece a unnest de vários arrays)
SELECT @administradora_id::text, @telefone::text,
       unnest(@chaves::text[]), unnest(@condominios::text[]), unnest(@textos::text[]),
       unnest(@fontes::text[]), unnest(@quandos::timestamptz[]), unnest(@confiancas::real[])
ON CONFLICT (administradora_id, telefone, chave) DO UPDATE
SET condominio_id = EXCLUDED.condominio_id,
    texto = EXCLUDED.texto,
    fonte = EXCLUDED.fonte,
    quando = EXCLUDED.quando,
    confianca = EXCLUDED.confianca;
