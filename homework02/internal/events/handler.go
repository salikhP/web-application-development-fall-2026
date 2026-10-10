package events

import (
	"encoding/json"
	"log"
	"net/http"
)

func ListHandler(repository *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := repository.ListLatest(r.Context(), 100)
		if err != nil {
			http.Error(
				w,
				"failed to load events",
				http.StatusInternalServerError,
			)

			log.Printf("failed to query Docker events: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(events); err != nil {
			log.Printf("failed to encode events: %v", err)
		}
	}
}
