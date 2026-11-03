-- por que não ent: acrescentar N eventos ignorando os já gravados num comando só
-- name: AcrescentarEventos :exec
INSERT INTO pedido_eventos (protocolo, seq, tipo, texto, por, quando)
-- várias unnest na lista do SELECT andam juntas, linha a linha (o sqlc não conhece a unnest de vários arrays)
SELECT unnest(@protocolos::text[]), unnest(@seqs::bigint[]), unnest(@tipos::text[]),
       unnest(@textos::text[]), unnest(@pors::text[]), unnest(@quandos::timestamptz[])
ON CONFLICT (protocolo, seq) DO NOTHING;
