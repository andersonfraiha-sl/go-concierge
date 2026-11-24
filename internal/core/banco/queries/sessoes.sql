-- name: LoadSession :batchone
SELECT s.* FROM sessions s JOIN channels c ON c.company_id = s.company_id AND c.id = s.channel_id
WHERE s.id = @id::text AND s.company_id = @company_id::text
 AND (@all_channels::boolean OR s.channel_id = @channel_id::uuid)
 AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM s.condo_id)
 AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM (s.data #>> '{scope,condo_id}')::integer);

-- name: SaveSession :batchone
INSERT INTO sessions AS current (id, company_id, channel_id, connector, protocol, phone_number, condo_id, data, updated_at)
SELECT @id::text, @company_id::text, @channel_id::uuid, @connector::text, @protocol::text,
 @phone_number::text, sqlc.narg(condo_id)::integer, @data::jsonb, @updated_at::timestamptz
FROM channels c WHERE c.company_id = @company_id::text AND c.id = @channel_id::uuid
 AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM sqlc.narg(condo_id)::integer)
 AND (c.condo_id IS NULL OR c.condo_id IS NOT DISTINCT FROM (@data::jsonb #>> '{scope,condo_id}')::integer)
ON CONFLICT (id) DO UPDATE SET phone_number = EXCLUDED.phone_number, condo_id = EXCLUDED.condo_id,
 data = EXCLUDED.data, updated_at = EXCLUDED.updated_at
WHERE current.company_id = EXCLUDED.company_id AND current.channel_id = EXCLUDED.channel_id
 AND current.connector = EXCLUDED.connector AND current.protocol = EXCLUDED.protocol
 AND current.phone_number = EXCLUDED.phone_number
RETURNING id;
