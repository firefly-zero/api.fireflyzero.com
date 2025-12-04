-- name: GetGroup :one
SELECT * FROM "groups" WHERE "slug" == $1;
