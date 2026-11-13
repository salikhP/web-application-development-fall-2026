package events

import (
	"context"
	"log"
	"time"

	"github.com/moby/moby/client"
)

func Listen(
	ctx context.Context,
	dockerClient *client.Client,
	repository *Repository,
) {
	result := dockerClient.Events(
		ctx,
		client.EventsListOptions{},
	)

	log.Println("Listening for Docker events...")

	for {
		select {
		case dockerEvent, ok := <-result.Messages:
			if !ok {
				return
			}

			if string(dockerEvent.Type) != "container" {
				continue
			}

			event := Event{
				ContainerID:    dockerEvent.Actor.ID,
				ContainerName:  dockerEvent.Actor.Attributes["name"],
				Image:          dockerEvent.Actor.Attributes["image"],
				Action:         string(dockerEvent.Action),
				ComposeProject: dockerEvent.Actor.Attributes["com.docker.compose.project"],
				ComposeService: dockerEvent.Actor.Attributes["com.docker.compose.service"],
				OccurredAt:     time.Unix(dockerEvent.Time, 0),
			}

			if err := repository.Save(ctx, event); err != nil {
				log.Printf(
					"failed to save Docker event: %v",
					err,
				)
				continue
			}

			shortID := event.ContainerID

			if len(shortID) > 12 {
				shortID = shortID[:12]
			}

			log.Printf(
				"[docker-event] time=%s container=%s id=%s image=%s action=%s",
				event.OccurredAt.Format(time.RFC3339),
				event.ContainerName,
				shortID,
				event.Image,
				event.Action,
			)

		case err, ok := <-result.Err:
			if ok && err != nil {
				log.Printf(
					"Docker event stream error: %v",
					err,
				)
			}

			return

		case <-ctx.Done():
			return
		}
	}
}
