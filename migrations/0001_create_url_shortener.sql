-- +goose Up
CREATE TABLE url_shorteners (
    id SERIAL PRIMARY KEY,
    original_url VARCHAR(2048) NOT NULL,
    short_code VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX idx_short_code ON url_shorteners (short_code);

INSERT INTO url_shorteners (original_url, short_code) VALUES
    ('https://www.google.com', 'google123'),
    ('https://www.github.com', 'git456'),
    ('https://www.youtube.com', 'yt789'),
    ('https://www.amazon.com', 'amzn101'),
    ('https://www.netflix.com', 'nflx202');


-- +goose Down
DROP INDEX idx_short_code;
DROP TABLE url_shorteners;
