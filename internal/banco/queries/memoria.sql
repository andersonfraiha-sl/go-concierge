-- por que não ent: gravar o contato, apagar as chaves que saíram e fazer upsert das restantes num comando só, atômico; o ent faria uma ida por fato e precisaria de uma transação com o sqlc
-- name: GravarFatos :exec
WITH contato AS (
    INSERT INTO memoria_contatos (administradora_id, telefone, nome, atualizada_em)
    VALUES (@administradora_id::text, @telefone::text, @nome::text, @atualizada_em::timestamptz)
    ON CONFLICT (administradora_id, telefone) DO UPDATE
    SET nome = EXCLUDED.nome,
        atualizada_em = EXCLUDED.atualizada_em
),
saidas AS (
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
