-- name: LoadRequests :batchmany
SELECT r.protocol, r.company_id, r.phone_number, r.condo_id, r.company_use_case_id, r.source_session_id, r.status, r.data, r.created_at, r.updated_at, c.use_case_id
FROM requests r JOIN company_use_cases c ON c.company_id = r.company_id AND c.id = r.company_use_case_id
WHERE r.company_id = @company_id::text AND r.phone_number = @phone_number::text
 AND (@all_condos::boolean OR r.condo_id IS NOT DISTINCT FROM sqlc.narg(condo_id)::integer);

-- name: SaveRequests :batchone
WITH incoming (protocol, condo_id, use_case_id, source_session_id, status, data, created_at, updated_at) AS (
 SELECT r->>'protocol', (r->>'condo_id')::integer, r->>'use_case_id', r->>'source_session_id', r->>'status', r->'data', (r->>'created_at')::timestamptz, (r->>'updated_at')::timestamptz
 FROM jsonb_array_elements(@requests::jsonb) r
), conflicts AS (
 SELECT r.protocol FROM requests r JOIN incoming i USING (protocol)
 WHERE r.company_id <> @company_id::text OR r.phone_number <> @phone_number::text
 OR r.condo_id IS DISTINCT FROM i.condo_id
), unresolved AS (
 SELECT i.protocol FROM incoming i
 LEFT JOIN company_use_cases c ON c.company_id = @company_id::text AND c.use_case_id = i.use_case_id
 WHERE c.id IS NULL
 OR i.source_session_id IS NOT NULL AND NOT EXISTS (
 SELECT 1 FROM sessions s WHERE s.id=i.source_session_id AND s.company_id= @company_id::text
 AND s.phone_number= @phone_number::text AND s.condo_id IS NOT DISTINCT FROM i.condo_id)
), removed_events AS (
 DELETE FROM request_events e USING requests r
 WHERE e.company_id = r.company_id AND e.protocol = r.protocol
 AND r.company_id = @company_id::text AND r.phone_number = @phone_number::text
 AND r.condo_id IS NOT DISTINCT FROM sqlc.narg(target_condo_id)::integer
 AND NOT EXISTS (SELECT 1 FROM incoming i WHERE i.protocol = r.protocol)
 AND NOT EXISTS (SELECT 1 FROM conflicts) AND NOT EXISTS (SELECT 1 FROM unresolved)
 RETURNING e.id
), removed AS (
 DELETE FROM requests r
 WHERE r.company_id = @company_id::text AND r.phone_number = @phone_number::text
 AND r.condo_id IS NOT DISTINCT FROM sqlc.narg(target_condo_id)::integer
 AND NOT EXISTS (SELECT 1 FROM incoming i WHERE i.protocol = r.protocol)
 AND NOT EXISTS (SELECT 1 FROM conflicts) AND NOT EXISTS (SELECT 1 FROM unresolved)
 AND (SELECT count(*) FROM removed_events) >= 0
), saved AS (
 INSERT INTO requests AS current (protocol, company_id, phone_number, condo_id, company_use_case_id, source_session_id, status, data, created_at, updated_at)
 SELECT i.protocol, @company_id::text, @phone_number::text, i.condo_id, c.id, i.source_session_id, i.status,
 i.data || jsonb_build_object('company_use_case_id', c.id), i.created_at, i.updated_at
 FROM incoming i JOIN company_use_cases c ON c.company_id = @company_id::text AND c.use_case_id = i.use_case_id
 WHERE NOT EXISTS (SELECT 1 FROM conflicts) AND NOT EXISTS (SELECT 1 FROM unresolved)
 ON CONFLICT (protocol) DO UPDATE SET status = EXCLUDED.status, data = EXCLUDED.data,
 source_session_id = EXCLUDED.source_session_id, updated_at = EXCLUDED.updated_at
 WHERE current.company_id = EXCLUDED.company_id AND current.phone_number = EXCLUDED.phone_number
 AND current.condo_id IS NOT DISTINCT FROM EXCLUDED.condo_id AND current.company_use_case_id = EXCLUDED.company_use_case_id
 RETURNING current.protocol
), events AS (
 INSERT INTO request_events (company_id, protocol, sequence, type, text, actor, occurred_at)
 SELECT @company_id::text, e->>'protocol', (e->>'sequence')::integer, e->>'type', e->>'text', e->>'actor', (e->>'occurred_at')::timestamptz
 FROM jsonb_array_elements(@events::jsonb) e
 JOIN saved s ON s.protocol = e->>'protocol'
 ON CONFLICT (company_id, protocol, sequence) DO NOTHING
)
SELECT ARRAY(SELECT protocol FROM conflicts)::text[] AS conflicts,
 ARRAY(SELECT protocol FROM unresolved)::text[] AS unresolved,
 ARRAY(SELECT protocol FROM incoming i WHERE NOT EXISTS (SELECT 1 FROM saved s WHERE s.protocol = i.protocol))::text[] AS missing;
