-- por que não ent: N linhas num comando só, dentro do lote pós resposta; o ent faria uma ida por linha
-- name: InserirAuditoria :batchexec
INSERT INTO ganchos_auditoria (
    id, criado_em, origem, evento, gancho, regra_id, regra_versao, decisao, motivo,
    administradora_id, condominio_id, sessao_id, protocolo, usuario_id, tela, ferramenta, quantidade
)
-- várias unnest na lista do SELECT andam juntas, linha a linha (o sqlc não conhece a unnest de vários arrays)
SELECT gen_random_uuid(),
       unnest(@criado_em::timestamptz[]), unnest(@origem::text[]), unnest(@evento::text[]),
       unnest(@gancho::text[]), unnest(@regra_id::text[]), unnest(@regra_versao::bigint[]),
       unnest(@decisao::text[]), unnest(@motivo::text[]), unnest(@administradora_id::text[]),
       unnest(@condominio_id::text[]), unnest(@sessao_id::text[]), unnest(@protocolo::text[]),
       unnest(@usuario_id::text[]), unnest(@tela::text[]), unnest(@ferramenta::text[]),
       unnest(@quantidade::bigint[]);
