-- por que não ent: apagar, fazer upsert só sobre linhas do contato, conferir os donos e
-- acrescentar os eventos numa ida ao banco, atômica; pelo ent seriam ~7 idas e os eventos
-- ficariam fora da transação (ent fala database/sql, sqlc fala pgx)
-- name: GravarPedidos :exec
SELECT gravar_pedidos(
    @administradora_id::text, @telefone::text,
    @protocolos::text[], @condominios::text[], @casos::text[], @estados::text[],
    @dados::text[], @criados::timestamptz[], @atualizados::timestamptz[],
    @evento_protocolos::text[], @evento_seqs::bigint[], @evento_tipos::text[],
    @evento_textos::text[], @evento_pors::text[], @evento_quandos::timestamptz[]
);
