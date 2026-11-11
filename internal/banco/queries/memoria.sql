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

-- por que não ent: o contato e os fatos numa query só, no lote que carrega o turno a frio
-- (sessão, memória e pedidos numa ida). Sem linha de contato não há fatos (GravarFatos grava
-- os dois).
-- name: LerMemoria :batchmany
SELECT c.nome, c.atualizada_em, f.chave, f.texto, f.fonte, f.condominio_id, f.quando, f.confianca
FROM memoria_contatos c
LEFT JOIN memoria_fatos f ON f.administradora_id = c.administradora_id AND f.telefone = c.telefone
WHERE c.administradora_id = @administradora_id::text AND c.telefone = @telefone::text
ORDER BY f.id;

-- por que não ent: entra no lote do turno a frio junto com LerMemoria (uma ida).
-- name: LerPadroes :batchmany
SELECT caso, caso_nome, condominio_id, unidade_id, valor, atualizado_em, suprimido_ate
FROM memoria_padroes
WHERE administradora_id = @administradora_id::text AND telefone = @telefone::text
ORDER BY id;

-- por que não ent: vários padrões num comando, no lote pós envio junto com a sessão e a carteira (uma ida); o contato é garantido na mesma ida para o Listar enxergá-lo.
-- name: GravarPadroes :batchexec
WITH contato AS (
    INSERT INTO memoria_contatos (administradora_id, telefone, atualizada_em)
    VALUES (@administradora_id::text, @telefone::text, @atualizada_em::timestamptz)
    ON CONFLICT (administradora_id, telefone) DO UPDATE
    SET atualizada_em = GREATEST(memoria_contatos.atualizada_em, EXCLUDED.atualizada_em)
)
INSERT INTO memoria_padroes (administradora_id, telefone, caso, caso_nome, condominio_id, unidade_id, valor, atualizado_em, suprimido_ate)
-- várias unnest na lista do SELECT andam juntas, linha a linha (o sqlc não conhece a unnest de vários arrays)
SELECT @administradora_id::text, @telefone::text,
       unnest(@casos::text[]), unnest(@nomes::text[]), unnest(@condominios::text[]), unnest(@unidades::text[]),
       unnest(@valores::float8[]), unnest(@atualizados::timestamptz[]), unnest(@suprimidos::timestamptz[])
ON CONFLICT (administradora_id, telefone, caso, condominio_id, unidade_id) DO UPDATE
SET caso_nome = EXCLUDED.caso_nome,
    valor = EXCLUDED.valor,
    atualizado_em = EXCLUDED.atualizado_em,
    suprimido_ate = EXCLUDED.suprimido_ate;

-- por que não ent: o decaimento é calculado por linha (power), que o ent não expressa.
-- name: LimparPadroes :execrows
DELETE FROM memoria_padroes
WHERE valor * power(0.5, extract(epoch FROM (sqlc.arg(agora)::timestamptz - atualizado_em)) / @meia_vida_segundos::float8) < @piso::float8;
