-- name: CreateOrderItem :one
INSERT INTO "order_items"
("order", "product", "quantity", "retail_price")
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListOrderItems :many
SELECT * FROM "order_items" WHERE "order" = $1;

-- name: GetOrderItem :one
SELECT * FROM "order_items"
WHERE "order" = $1 AND "id" = $2;

-- name: SetOrderItemRetailPrice :one
UPDATE "order_items"
SET "retail_price" = $1
WHERE "order" = $2 AND "id" = $3
RETURNING *;

-- name: SetOrderItemQuantity :one
UPDATE "order_items"
SET "quantity" = $1
WHERE "order" = $2 AND "id" = $3
RETURNING *;

-- name: DeleteOrderItem :exec
DELETE FROM "order_items"
WHERE "order" = $1 AND "id" = $2;
