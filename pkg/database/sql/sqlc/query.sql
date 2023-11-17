-- name: GetUser :one
SELECT *
FROM users
WHERE discord_tag = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByToken :one
SELECT *
FROM users
WHERE access_token = $1
LIMIT 1;

-- name: SetUsername :exec
UPDATE users
SET username = $2
WHERE id = $1;

-- name: SetAccessToken :exec
UPDATE users
SET access_token = $2
WHERE id = $1;

-- name: UpsertUser :one
INSERT
INTO users (discord_id, discord_tag, avatar, settings)
VALUES ($1, $2, $3, $4)
ON CONFLICT (discord_id)
    DO UPDATE SET discord_tag = excluded.discord_tag,
                  avatar      = excluded.avatar
RETURNING *;

-- name: DeleteUser :exec
DELETE
FROM users
WHERE id = $1;

-- name: InsertEncounter :one
INSERT
INTO encounters (uploaded_by, settings, tags, header, data, difficulty, boss, date, duration, local_player)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: ProcessEncounter :exec
UPDATE encounters
SET header = $2,
    data   = $3
WHERE id = $1;

-- name: GetEncounter :one
SELECT *
FROM encounters
WHERE id = $1
LIMIT 1;

-- name: ListRecentEncounters :many
SELECT id,
       difficulty,
       uploaded_by,
       uploaded_at,
       settings,
       tags,
       header,
       boss,
       date,
       duration,
       local_player
FROM encounters
WHERE (sqlc.narg('date')::TIMESTAMP IS NULL
    OR (sqlc.narg('date') > date OR (sqlc.narg('date')::TIMESTAMP = date AND sqlc.narg('id')::INT < id)))
  AND (sqlc.narg('user')::UUID IS NULL
    OR sqlc.narg('user') = uploaded_by)
ORDER BY date DESC, id ASC
LIMIT 5;

-- name: GetData :one
SELECT data
FROM encounters
WHERE id = $1;

-- name: GetRaidStats :many
SELECT boss, difficulty, count(*)
FROM encounters
GROUP BY boss, difficulty;

-- name: GetUniqueUploaders :one
SELECT COUNT(DISTINCT jsonb_object_keys(header -> 'players'))
FROM encounters;

-- name: CountClasses :many
SELECT (value ->> 'class')::STRING AS class, COUNT(*)
FROM encounters,
     jsonb_each(header -> 'players') AS player
GROUP BY (value ->> 'class')::STRING;