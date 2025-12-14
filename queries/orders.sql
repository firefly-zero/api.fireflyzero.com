-- name: EnsureOrder :one
INSERT INTO "orders" ("user") VALUES ($1)
ON CONFLICT ("user") DO NOTHING
RETURNING *;

-- name: GetDraftOrder :one
SELECT * FROM "orders"
WHERE "user" = $1 AND "status" = 'draft';

-- name: GetOrder :one
SELECT * FROM "orders"
WHERE "user" = $1 AND "id" = $2;

-- name: SetOrderStatus :one
UPDATE "orders"
SET "status" = $1, "updated_at" = NOW()
WHERE "id" = $2 AND "user" = $3
RETURNING *;

-- name: MarkOrderPaid :one
--
-- Mark the order as paid and set the Stripe session ID.
UPDATE "orders"
SET
    "paid" = true,
    "stripe_id" = $1,
    "updated_at" = NOW()
WHERE "id" = $2
RETURNING *;
