package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/moby/moby/client"

	"homework02/internal/database"
	dockerapi "homework02/internal/docker"
	"homework02/internal/events"
)

type healthResponse struct {
	Status string `json:"status"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dockerClient, err := client.New(client.FromEnv)
	if err != nil {
		log.Fatalf(
			"failed to create Docker client: %v",
			err,
		)
	}
	defer dockerClient.Close()

	db, err := database.Connect(ctx)
	if err != nil {
		log.Fatalf(
			"failed to initialize database: %v",
			err,
		)
	}
	defer db.Close()

	log.Println("Connected to PostgreSQL")
	log.Println("Database initialized")

	eventRepository := events.NewRepository(db)

	go events.Listen(
		ctx,
		dockerClient,
		eventRepository,
	)

	dockerHandler := dockerapi.NewHandler(dockerClient)

	mux := http.NewServeMux()

	mux.HandleFunc(
		"GET /health",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			status := "ok"

			if err := db.Ping(r.Context()); err != nil {
				status = "degraded"
				w.WriteHeader(
					http.StatusServiceUnavailable,
				)
			}

			if err := json.NewEncoder(w).Encode(
				healthResponse{
					Status: status,
				},
			); err != nil {
				log.Printf(
					"failed to encode response: %v",
					err,
				)
			}
		},
	)

	mux.HandleFunc(
		"GET /events",
		events.ListHandler(eventRepository),
	)

	dockerHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println(
		"DockTrace API listening on :8080",
	)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
