-- name: InserirAuditoria :batchexec
INSERT INTO hook_audit_logs (
    id, created_at, origin, event, hook, rule_id, rule_version, decision, reason,
    company_id, condo_id, session_id, protocol, user_id, screen, tool, count, scope_type, channel_id
)
SELECT gen_random_uuid(), a.created_at, a.origin, a.event, a.hook,
       NULLIF(a.rule_id, '')::uuid, a.rule_version, a.decision, a.reason,
       NULLIF(a.company_id, ''), NULLIF(NULLIF(a.condo_id, ''), '*')::integer,
       NULLIF(a.session_id, ''), a.protocol, a.user_id, a.screen, a.tool, a.count,
       CASE WHEN a.company_id = '' THEN 'GLOBAL' ELSE 'COMPANY' END,
       NULLIF(a.channel_id, '')::uuid
FROM (
 SELECT unnest(@criado_em::timestamptz[]) AS created_at,
        unnest(@origem::text[]) AS origin, unnest(@evento::text[]) AS event,
        unnest(@gancho::text[]) AS hook, unnest(@regra_id::text[]) AS rule_id,
        unnest(@regra_versao::integer[]) AS rule_version, unnest(@decisao::text[]) AS decision,
        unnest(@motivo::text[]) AS reason, unnest(@administradora_id::text[]) AS company_id,
        unnest(@condominio_id::text[]) AS condo_id, unnest(@sessao_id::text[]) AS session_id,
        unnest(@protocolo::text[]) AS protocol, unnest(@usuario_id::text[]) AS user_id,
        unnest(@tela::text[]) AS screen, unnest(@ferramenta::text[]) AS tool,
        unnest(@quantidade::integer[]) AS count,
        unnest(@channel_id::text[]) AS channel_id
) a;
