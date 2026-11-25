-- name: ListInstanceIPBlocks :many
SELECT id, cidr, note, created_at FROM instance_ip_blocks ORDER BY created_at, id;

-- name: CreateInstanceIPBlock :execrows
INSERT INTO instance_ip_blocks (id, cidr, note, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (cidr) DO NOTHING;

-- name: DeleteInstanceIPBlock :execrows
DELETE FROM instance_ip_blocks WHERE id = $1;
