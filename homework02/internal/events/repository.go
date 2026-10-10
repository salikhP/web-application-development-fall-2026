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
	rows, err := r.db.Query(
		ctx,
		`
			SELECT
				id,
				container_id,
				container_name,
				image,
				action,
				compose_project,
				compose_service,
				occurred_at
			FROM docker_events
			ORDER BY occurred_at DESC
			LIMIT $1
		`,
		limit,
	)
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
