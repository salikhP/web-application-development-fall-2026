# Homework 03 - DockTrace

DockTrace records Docker container lifecycle events in PostgreSQL and exposes
container history and resource usage through a REST API. Homework 03 adds a
catalog for Compose projects, services, and operational incidents.

- A project can have many services. Each service belongs to one project.
- A service can have many incidents. Each incident belongs to one service.
- Docker events are recorded independently of the catalog. A service's event
  feed matches its current project and service names against the labels on
  captured events. Renaming a catalog entry changes which events match.

The backend uses Go's HTTP router and parameterized SQL through pgx. Docker
Compose runs the backend and PostgreSQL as separate containers.

## Requirements

- Docker Engine or Docker Desktop with Docker Compose
- Postman for the desktop test workflow, or Docker alone for the command-line workflow

## Configuration

Run these commands from the repository root:

```sh
cd homework03
cp .env.example .env
```

Set a local `POSTGRES_PASSWORD` in `.env` before starting for the first time.
`API_PORT` defaults to `18083`. PostgreSQL is reachable by the backend over the
Compose network. The backend uses the Docker socket to read container events
and stats, and the API binds to the host loopback interface.

## Running the application

```sh
docker compose up --build --wait
docker compose ps
docker compose exec -T backend wget -qO- http://127.0.0.1:8080/health
```

The health check returns `{"status":"ok"}` when PostgreSQL is reachable. View
backend logs with `docker compose logs -f backend`. Stop the stack with
`docker compose down`. The named PostgreSQL volume preserves data across restarts
and ordinary shutdowns. Homework 03 uses its own Compose volume.

## API

The default base URL is `http://localhost:18083`. There are no query parameters.
IDs in catalog paths are positive 64-bit integers. A dash in the body column
means no request body.

| Method | Path | Path parameters | JSON request body | Success |
| --- | --- | --- | --- | --- |
| GET | `/health` | - | - | `200` status object |
| GET | `/containers` | - | - | `200` Docker container array |
| GET | `/containers/{id}/stats` | Docker container ID or name | - | `200` container stats object |
| GET | `/events` | - | - | `200` latest 100 Docker events |
| GET | `/projects` | - | - | `200` project array |
| POST | `/projects` | - | `{"name":"homework03"}` | `201` project |
| GET | `/projects/{id}` | Project ID | - | `200` project |
| PUT | `/projects/{id}` | Project ID | `{"name":"docktrace"}` | `200` updated project |
| DELETE | `/projects/{id}` | Project ID | - | `204` empty body |
| GET | `/projects/{id}/services` | Project ID | - | `200` service array |
| POST | `/projects/{id}/services` | Project ID | `{"name":"backend"}` | `201` service |
| GET | `/services/{id}` | Service ID | - | `200` service |
| PUT | `/services/{id}` | Service ID | `{"name":"api"}` | `200` updated service |
| DELETE | `/services/{id}` | Service ID | - | `204` empty body |
| GET | `/services/{id}/events` | Service ID | - | `200` latest 100 matching Docker events |
| GET | `/services/{id}/incidents` | Service ID | - | `200` incident array |
| POST | `/services/{id}/incidents` | Service ID | `{"title":"Container restarted","status":"open","details":"Check event history."}` | `201` incident |
| GET | `/incidents/{id}` | Incident ID | - | `200` incident |
| PUT | `/incidents/{id}` | Incident ID | `{"title":"Container restarted","status":"resolved","details":"Restart verified."}` | `200` updated incident |
| DELETE | `/incidents/{id}` | Incident ID | - | `204` empty body |

### Request validation and errors

Send `Content-Type: application/json` for POST and PUT. PUT replaces all writable
fields. Parent IDs come from the creation URL. Project names are unique and
use 1–100 lowercase letters, digits, underscores, or hyphens, starting with a
letter or digit. Service names are unique within a project and use 1–100
letters, digits, periods, underscores, or hyphens. Incident titles require
1–200 characters. Incident status is `open`, `investigating`, or `resolved`.
Optional `details` accepts up to 4000 characters and defaults to an empty
string.

Catalog errors have the form `{"error":"message"}`.

| Status | Condition |
| --- | --- |
| `400 Bad Request` | Invalid catalog ID, malformed JSON, unknown fields, or failed field validation |
| `404 Not Found` | Catalog record or required parent does not exist |
| `409 Conflict` | Duplicate project/service name, or deletion attempted while child records exist |
| `413 Content Too Large` | Request body exceeds 64 KiB |
| `415 Unsupported Media Type` | POST or PUT lacks the JSON content type |
| `500 Internal Server Error` | Database or Docker Engine request failed. The original Docker and event endpoints return plain-text errors |
| `503 Service Unavailable` | Health check cannot reach PostgreSQL and returns `{"status":"degraded"}` |

### Response examples

A successful POST returns the created record and a `Location` header pointing
to its item URL. Catalog lists are ordered by ID and return `[]` when empty.
Event lists are ordered newest first and also return `[]` when empty.

**Project, service, and incident objects**

```json
{"id":1,"name":"homework03"}
```

```json
{"id":2,"project_id":1,"name":"backend"}
```

```json
{"id":3,"service_id":2,"title":"Container restarted","status":"open","details":"Check event history."}
```

**Docker container and stats objects**

```json
{"id":"4faa358abc8d","name":"homework03-backend-1","image":"homework03-backend","state":"running","status":"Up 2 minutes (healthy)"}
```

```json
{"container_id":"4faa358abc8d","container_name":"/homework03-backend-1","cpu_percent":0.1,"memory_usage":7340032,"memory_limit":1073741824,"memory_percent":0.68}
```

Stats report memory in bytes. Query a running container for live stats.

**Docker event object**

```json
{"id":12,"container_id":"4faa358abc8d1be4e37cebe9cac67c304df0cad110be9109afa506b4e9c7bb7a","container_name":"homework03-backend-1","image":"homework03-backend","action":"start","compose_project":"homework03","compose_service":"backend","occurred_at":"2026-11-12T12:00:00Z"}
```

Compose fields are omitted when the event has no corresponding labels.

## Testing

### Postman

1. Start the stack and check its health using the commands above.
2. Import [DockTrace.postman_collection.json](DockTrace.postman_collection.json).
3. If you changed `API_PORT`, update the collection's `baseUrl` variable to match.
4. Open the Collection Runner and run the full collection in order. It captures
   IDs from create responses, verifies the nested resources and error responses,
   and removes the records it created.

### Containerized command-line test

From `homework03`, run the same collection using Newman in a container. The
example assumes the default Compose project name, `homework03`.

```sh
docker run --rm --network homework03_default \
  --mount "type=bind,source=$PWD,target=/work,readonly" \
  node:22-alpine npx --yes newman@6.2.1 run /work/DockTrace.postman_collection.json \
  --env-var baseUrl=http://backend:8080
```

### Generate real events

With the stack running, start a short-lived container for the Compose
`backend` service. The listener records its lifecycle events.

```sh
docker compose run --no-deps --rm --entrypoint /bin/true backend
docker compose exec -T backend wget -qO- http://127.0.0.1:8080/events
```

To check filtering in Postman, create a project named `homework03` and a service
named `backend`. Use the returned service ID in `GET /services/{id}/events`.
Events from the short-lived container should appear with both matching Compose
labels. If you use a custom Compose project name, use that name in the catalog.

## Submission

Push the `homework03` directory to [the GitHub repository](https://github.com/salikhP/web-application-development-fall-2026),
attach the repository link to the assignment, and press **Turn In** before the
deadline.
