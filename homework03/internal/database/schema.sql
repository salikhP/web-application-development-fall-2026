CREATE TABLE IF NOT EXISTS docker_events (
    id BIGSERIAL PRIMARY KEY,
    container_id TEXT NOT NULL,
    container_name TEXT,
    image TEXT,
    action TEXT NOT NULL,
    compose_project TEXT,
    compose_service TEXT,
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS docker_events_service_time_idx
    ON docker_events (compose_project, compose_service, occurred_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS projects (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE CHECK (char_length(name) BETWEEN 1 AND 100)
);

CREATE TABLE IF NOT EXISTS services (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    UNIQUE (project_id, name)
);

CREATE TABLE IF NOT EXISTS incidents (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    status TEXT NOT NULL CHECK (status IN ('open', 'investigating', 'resolved')),
    details TEXT NOT NULL DEFAULT '' CHECK (char_length(details) <= 4000)
);

CREATE INDEX IF NOT EXISTS incidents_service_id_idx ON incidents (service_id, id);
