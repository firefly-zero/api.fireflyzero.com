-- name: CreateOrderItem :one
INSERT INTO "order_items"
("order", "product", "release", "quantity", "retail_price")
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT ("user") DO NOTHING
RETURNING *;

-- name: ListOrderItems :many
SELECT * FROM "order_items" WHERE "order" = $1;
