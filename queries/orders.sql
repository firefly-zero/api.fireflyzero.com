-- name: EnsureOrder :one
INSERT INTO "orders" ("user") VALUES ($1)
ON CONFLICT ("user") DO NOTHING
RETURNING *;

-- name: GetDraftOrder :one
SELECT * FROM "orders"
WHERE "user" = $1 AND "status" = 'draft';
