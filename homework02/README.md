# Homework 02 - DockTrace

DockTrace is a lightweight flight recorder for Docker containers.

Docker provides real-time container lifecycle events, but those events are not intended to be a persistent historical record. 
DockTrace listens to Docker Engine events, stores them in PostgreSQL, and exposes the recorded history through a simple HTTP API.

The entire application runs as a multi-container Docker Compose project.

## Requirements

- Docker
- Docker Compose

Docker Desktop can be used on macOS and Windows.

## Configuration

Copy the example environment file:

```shell
cp .env.example .env
```
Change `POSTGRES_PASSWORD` to your own local password.

## Running the Application

From the project directory:

```shell
docker compose up
```

To explicitly rebuild the backend image:

```shell
docker compose up --build
```

Run in detached mode:

```shell
docker compose up -d
```

Check running services:
```shell
docker compose ps
```

## API

### Health Check `GET /health`
 
```shell
curl -s localhost:8080/health | jq
```

Response:

```json
{
"status": "ok"
}
```

The endpoint also verifies that the backend can connect to PostgreSQL.

### List Docker Containers `GET /containers`

Example:
```shell
curl -s localhost:8080/containers | jq
```

Example response:

```json
[
    {
    "id": "4faa358abc8d",
    "name": "homework02-postgres-1",
    "image": "postgres:17-alpine",
    "state": "running",
    "status": "Up 2 minutes (healthy)"
    }
]
```


### View Recorded Docker Events `GET /events`

Example:

```shell
curl -s localhost:8080/events | jq
```


Example event:
```json
{
    "id": 12,
    "container_id": "4faa358abc8d1be4e37cebe9cac67c304df0cad110be9109afa506b4e9c7bb7a",
    "container_name": "homework02-postgres-1",
    "image": "postgres:17-alpine",
    "action": "start",
    "compose_project": "homework02",
    "compose_service": "postgres",
    "occurred_at": "2026-10-09T21:30:59+05:00"
}

```

The latest 100 recorded events are returned.

### Container Resource Statistics `GET /containers/{id}/stats`

The {id} can also be a Docker container name.

Example:
```shell
curl -s localhost:8080/containers/homework02-postgres-1/stats | jq
```

The endpoint returns current CPU and memory usage obtained from Docker Engine.

### Generating Docker Events

DockTrace listens to real Docker Engine events.

For example:

```shell
docker create --name docktrace-test postgres:17-alpine
docker start docktrace-test
docker stop docktrace-test
docker rm docktrace-test
```

Then inspect the recorded events:

```shell
curl -s localhost:8080/events | jq
```

The events are persisted in PostgreSQL rather than existing only in the live Docker event stream.
