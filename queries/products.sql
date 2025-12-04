-- name: GetProduct :one
SELECT * FROM "products" WHERE "slug" == $1;
