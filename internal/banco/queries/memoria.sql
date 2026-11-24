-- name: LoadMemory :batchmany
SELECT c.name, c.updated_at, f.key, f.text, f.source, f.condo_id, f.occurred_at, f.confidence
FROM contact_memories c
LEFT JOIN memory_facts f ON f.company_id = c.company_id AND f.phone_number = c.phone_number
 AND (@all_condos::boolean OR f.condo_id IS NOT DISTINCT FROM sqlc.narg(condo_id)::integer)
WHERE c.company_id = @company_id::text AND c.phone_number = @phone_number::text
ORDER BY f.id;

-- name: SaveMemory :exec
WITH contact AS (
 INSERT INTO contact_memories (company_id, phone_number, name, updated_at)
 VALUES (@company_id::text, @phone_number::text, @name::text, @updated_at::timestamptz)
 ON CONFLICT (company_id, phone_number) DO UPDATE SET name = EXCLUDED.name, updated_at = EXCLUDED.updated_at
 RETURNING company_id
), incoming (condo_id, key, text, source, occurred_at, confidence) AS (
 SELECT (f->>'condo_id')::integer, f->>'key', f->>'text', f->>'source', (f->>'occurred_at')::timestamptz, (f->>'confidence')::real
 FROM jsonb_array_elements(@facts::jsonb) f
), removed AS (
 DELETE FROM memory_facts f
 WHERE f.company_id = @company_id::text AND f.phone_number = @phone_number::text
 AND f.condo_id IS NOT DISTINCT FROM sqlc.narg(target_condo_id)::integer
 AND NOT EXISTS (SELECT 1 FROM incoming i WHERE i.condo_id IS NOT DISTINCT FROM f.condo_id AND i.key = f.key)
)
INSERT INTO memory_facts (company_id, phone_number, condo_id, key, text, source, occurred_at, confidence)
SELECT @company_id::text, @phone_number::text, i.condo_id, i.key, i.text, i.source, i.occurred_at, i.confidence
FROM incoming i, contact
ON CONFLICT (company_id, phone_number, condo_id, key) DO UPDATE
SET text = EXCLUDED.text, source = EXCLUDED.source, occurred_at = EXCLUDED.occurred_at, confidence = EXCLUDED.confidence;
