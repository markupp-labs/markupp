-- name: ListInstanceRegistrationMethods :many
SELECT method, open FROM instance_registration_methods;

-- name: SaveInstanceRegistrationMethod :exec
INSERT INTO instance_registration_methods (method, open) VALUES ($1, $2)
ON CONFLICT (method) DO UPDATE SET open = EXCLUDED.open, updated_at = now();
