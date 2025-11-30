-- name: CreateUser :one
INSERT INTO "users"
("name", "email", "country", "language", "timezone")
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM "users" WHERE "email" = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM "users" WHERE "id" = $1 LIMIT 1;

-- name: GetUserByUsernameI :one
--
-- Do case-insensitive search of user by username.
SELECT * FROM "users" WHERE LOWER("name") = LOWER(@name) LIMIT 1;

-- name: ListUsernames :many
SELECT "id", "name" FROM "users";

-- name: UpdateUser :one
UPDATE "users"
SET
    "country"           = COALESCE(sqlc.narg(country),          "country"),
    "language"          = COALESCE(sqlc.narg(language),         "language"),
    "timezone"          = COALESCE(sqlc.narg(timezone),         "timezone"),

    "updated_at"    = NOW()
WHERE "id" = @id
RETURNING *;

-- name: SoftDeleteUser :exec
--
-- Schedule the user to be deleted later.
UPDATE "users" SET "deleted_at" = @now WHERE "id" = @id;

-- name: DeleteDeletedUsers :exec
--
-- Delete from DB old users marked as soft-deleted.
DELETE FROM "users"
WHERE "deleted_at" IS NOT NULL
AND "deleted_at" < @before;
