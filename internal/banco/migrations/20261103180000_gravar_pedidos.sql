-- +goose Up
-- Escrita à mão (o Atlas só gera tabelas e índices): grava a carteira inteira de um contato
-- numa ida ao banco. Apaga os pedidos que saíram, faz upsert dos restantes só sobre linhas do
-- próprio contato e acrescenta os eventos novos. Se algum protocolo já é de outro contato, o
-- upsert o pula, a contagem não fecha e o RAISE desfaz tudo com o SQLSTATE CC001.
-- +goose StatementBegin
CREATE FUNCTION gravar_pedidos(
    p_administradora_id text, p_telefone text,
    p_protocolos text[], p_condominios text[], p_casos text[], p_estados text[],
    p_dados text[], p_criados timestamptz[], p_atualizados timestamptz[],
    e_protocolos text[], e_seqs bigint[], e_tipos text[], e_textos text[], e_pors text[],
    e_quandos timestamptz[]
) RETURNS void LANGUAGE plpgsql AS $$
DECLARE
    gravados bigint;
BEGIN
    DELETE FROM pedidos
    WHERE administradora_id = p_administradora_id
      AND telefone = p_telefone
      AND protocolo <> ALL(p_protocolos);

    INSERT INTO pedidos AS atual (protocolo, administradora_id, condominio_id, telefone, caso, estado,
                                  dados, criado_em, atualizado_em)
    SELECT l.protocolo, p_administradora_id, l.condominio_id, p_telefone, l.caso, l.estado,
           l.dados::jsonb, l.criado_em, l.atualizado_em
    FROM unnest(p_protocolos, p_condominios, p_casos, p_estados, p_dados, p_criados, p_atualizados)
         AS l(protocolo, condominio_id, caso, estado, dados, criado_em, atualizado_em)
    ON CONFLICT (protocolo) DO UPDATE
    SET condominio_id = EXCLUDED.condominio_id,
        caso = EXCLUDED.caso,
        estado = EXCLUDED.estado,
        dados = EXCLUDED.dados,
        criado_em = EXCLUDED.criado_em,
        atualizado_em = EXCLUDED.atualizado_em
    WHERE atual.administradora_id = EXCLUDED.administradora_id
      AND atual.telefone = EXCLUDED.telefone;
    GET DIAGNOSTICS gravados = ROW_COUNT;
    IF gravados <> cardinality(p_protocolos) THEN
        RAISE EXCEPTION 'protocolo já em uso por outro contato' USING ERRCODE = 'CC001';
    END IF;

    INSERT INTO pedido_eventos (protocolo, seq, tipo, texto, por, quando)
    SELECT * FROM unnest(e_protocolos, e_seqs, e_tipos, e_textos, e_pors, e_quandos)
    ON CONFLICT (protocolo, seq) DO NOTHING;
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION gravar_pedidos;
