package events

import "time"

type Event struct {
	ID             int64     `json:"id"`
	ContainerID    string    `json:"container_id"`
	ContainerName  string    `json:"container_name"`
	Image          string    `json:"image"`
	Action         string    `json:"action"`
	ComposeProject string    `json:"compose_project,omitempty"`
	ComposeService string    `json:"compose_service,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
}
