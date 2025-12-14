-- name: CreateUser :one
INSERT INTO "users"
("email", "country", "language", "timezone")
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM "users" WHERE "email" = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM "users" WHERE "id" = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE "users"
SET
    "country"           = COALESCE(sqlc.narg(country),          "country"),
    "language"          = COALESCE(sqlc.narg(language),         "language"),
    "timezone"          = COALESCE(sqlc.narg(timezone),         "timezone"),
    "stripe_id"         = COALESCE(sqlc.narg(stripe_id),        "stripe_id"),

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
