-- por que não ent: entra no pgx.Batch das escritas depois da resposta, junto com os pedidos
-- name: SalvarSessao :exec
INSERT INTO sessoes (id, administradora_id, condominio_id, protocolo, atualizada_em, dados)
VALUES (@id::text, @administradora_id::text, @condominio_id::text, @protocolo::text,
        @atualizada_em::timestamptz, @dados::jsonb)
ON CONFLICT (id) DO UPDATE
SET administradora_id = EXCLUDED.administradora_id,
    condominio_id = EXCLUDED.condominio_id,
    protocolo = EXCLUDED.protocolo,
    atualizada_em = EXCLUDED.atualizada_em,
    dados = EXCLUDED.dados;
