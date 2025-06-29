-- +goose Up
CREATE TABLE companies (
                           id           SERIAL PRIMARY KEY,
                           name         TEXT NOT NULL,
                           email        TEXT NOT NULL UNIQUE,
                           password_hash TEXT NOT NULL,
                           api_key      TEXT UNIQUE,
                           client_id    TEXT UNIQUE,
                           created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
                           updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE companies;
