package events

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) Save(ctx context.Context, event Event) error {
	_, err := r.db.Exec(
		ctx,
		`
			INSERT INTO docker_events (
				container_id,
				container_name,
				image,
				action,
				compose_project,
				compose_service,
				occurred_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
		event.ContainerID,
		event.ContainerName,
		event.Image,
		event.Action,
		event.ComposeProject,
		event.ComposeService,
		event.OccurredAt,
	)

	return err
}

func (r *Repository) ListLatest(ctx context.Context, limit int) ([]Event, error) {
	return r.list(ctx, "ORDER BY occurred_at DESC, id DESC LIMIT $1", limit)
}

func (r *Repository) ListForService(ctx context.Context, project, service string) ([]Event, error) {
	return r.list(ctx, `
		WHERE compose_project = $1 AND compose_service = $2
		ORDER BY occurred_at DESC, id DESC
		LIMIT 100
	`, project, service)
}

func (r *Repository) list(ctx context.Context, filter string, args ...any) ([]Event, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			container_id,
			COALESCE(container_name, ''),
			COALESCE(image, ''),
			action,
			COALESCE(compose_project, ''),
			COALESCE(compose_service, ''),
			occurred_at
		FROM docker_events
	`+filter, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Event, 0)

	for rows.Next() {
		var event Event

		if err := rows.Scan(
			&event.ID,
			&event.ContainerID,
			&event.ContainerName,
			&event.Image,
			&event.Action,
			&event.ComposeProject,
			&event.ComposeService,
			&event.OccurredAt,
		); err != nil {
			return nil, err
		}

		result = append(result, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
