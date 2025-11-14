-- +goose Up
-- +goose StatementBegin
CREATE TYPE pull_request_status AS ENUM ('OPEN', 'MERGED');

CREATE TABLE IF NOT EXISTS team
(
    name VARCHAR(255) PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users
(
    id        VARCHAR(255) PRIMARY KEY,
    username  VARCHAR(100) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    team_name VARCHAR(255) REFERENCES team (name)
);

CREATE TABLE IF NOT EXISTS pull_request
(
    id                  VARCHAR(255) PRIMARY KEY,
    name                VARCHAR(255)                       NOT NULL,
    author_id           VARCHAR(255) REFERENCES users (id) NOT NULL,
    status              pull_request_status DEFAULT 'OPEN',
    need_more_reviewers BOOLEAN             DEFAULT FALSE,
    created_at          TIMESTAMPTZ         DEFAULT NOW(),
    merged_at           TIMESTAMPTZ         DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS review
(
    user_id         VARCHAR(255) REFERENCES users (id),
    pull_request_id VARCHAR(255) REFERENCES pull_request (id),
    PRIMARY KEY (user_id, pull_request_id)
);



-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS team CASCADE;
DROP TABLE IF EXISTS review CASCADE;
DROP TABLE IF EXISTS pull_request CASCADE;
DROP TYPE IF EXISTS pull_request_status;
-- +goose StatementEnd
