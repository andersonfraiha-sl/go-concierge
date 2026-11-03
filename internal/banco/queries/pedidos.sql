-- por que não ent: entra no lote que carrega o turno a frio (sessão, memória e pedidos numa ida)
-- name: LerPedidos :batchmany
SELECT protocolo, dados FROM pedidos
WHERE administradora_id = @administradora_id::text AND telefone = @telefone::text;

-- por que não ent: grava a carteira inteira numa ida, atômica, com a checagem de dono dos
-- protocolos; pelo ent seriam ~7 idas e os eventos ficariam fora da transação.
-- Um comando só: se algum protocolo é de outro contato (alheios), nenhuma escrita acontece
-- (todas pedem NOT EXISTS alheios) e quem chama redesenha esses protocolos e grava de novo.
-- O upsert só sobrescreve linha do próprio contato; um protocolo que outro contato gravou
-- entre a foto e o upsert volta em faltando. Os eventos só entram para pedidos gravados.
-- name: GravarPedidos :batchone
WITH alheios AS (
    SELECT protocolo FROM pedidos
    WHERE protocolo = ANY(@protocolos::text[])
      AND (administradora_id <> @administradora_id::text OR telefone <> @telefone::text)
),
apagados AS (
    DELETE FROM pedidos
    WHERE administradora_id = @administradora_id::text
      AND telefone = @telefone::text
      AND protocolo <> ALL(@protocolos::text[])
      AND NOT EXISTS (SELECT 1 FROM alheios)
),
gravados AS (
    INSERT INTO pedidos AS atual (protocolo, administradora_id, condominio_id, telefone, caso, estado,
                                  dados, criado_em, atualizado_em)
    -- várias unnest na lista do SELECT andam juntas, linha a linha (o sqlc não conhece a unnest de vários arrays)
    SELECT unnest(@protocolos::text[]), @administradora_id::text, unnest(@condominios::text[]), @telefone::text,
           unnest(@casos::text[]), unnest(@estados::text[]), unnest(@dados::text[])::jsonb,
           unnest(@criados::timestamptz[]), unnest(@atualizados::timestamptz[])
    WHERE NOT EXISTS (SELECT 1 FROM alheios)
    ON CONFLICT (protocolo) DO UPDATE
    SET condominio_id = EXCLUDED.condominio_id,
        caso = EXCLUDED.caso,
        estado = EXCLUDED.estado,
        dados = EXCLUDED.dados,
        criado_em = EXCLUDED.criado_em,
        atualizado_em = EXCLUDED.atualizado_em
    WHERE atual.administradora_id = EXCLUDED.administradora_id
      AND atual.telefone = EXCLUDED.telefone
    RETURNING atual.protocolo
),
eventos AS (
    INSERT INTO pedido_eventos (protocolo, seq, tipo, texto, por, quando)
    SELECT e.protocolo, e.seq, e.tipo, e.texto, e.por, e.quando
    FROM (SELECT unnest(@evento_protocolos::text[]) AS protocolo, unnest(@evento_seqs::bigint[]) AS seq,
                 unnest(@evento_tipos::text[]) AS tipo, unnest(@evento_textos::text[]) AS texto,
                 unnest(@evento_pors::text[]) AS por, unnest(@evento_quandos::timestamptz[]) AS quando) AS e
    WHERE e.protocolo IN (SELECT protocolo FROM gravados)
    ON CONFLICT (protocolo, seq) DO NOTHING
)
SELECT ARRAY(SELECT protocolo FROM alheios)::text[] AS alheios,
       ARRAY(SELECT p FROM unnest(@protocolos::text[]) AS p
             WHERE NOT EXISTS (SELECT 1 FROM alheios)
               AND p NOT IN (SELECT protocolo FROM gravados))::text[] AS faltando;
