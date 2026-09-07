CREATE TYPE readable_status AS ENUM ('pending', 'succeeded', 'failed');
CREATE TYPE readable_format AS ENUM ('pdf', 'epub', 'html');

CREATE TABLE IF NOT EXISTS readables (
    id BIGINT PRIMARY KEY,
    status readable_status NOT NULL DEFAULT 'pending',
    format readable_format NOT NULL,
    created TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
