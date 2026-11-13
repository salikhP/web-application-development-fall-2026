package catalog

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"homework03/internal/events"
)

type Project struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Service struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id" db:"project_id"`
	Name      string `json:"name"`
}

type Incident struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id" db:"service_id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Details   string `json:"details"`
}

func (p Project) resourceID() int64  { return p.ID }
func (s Service) resourceID() int64  { return s.ID }
func (i Incident) resourceID() int64 { return i.ID }

type nameInput struct {
	Name string `json:"name"`
}

type incidentInput struct {
	Title   string `json:"title"`
	Status  string `json:"status"`
	Details string `json:"details"`
}

var (
	projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,99}$`)
	serviceName = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,100}$`)
)

func RegisterRoutes(mux *http.ServeMux, db *pgxpool.Pool, eventRepository *events.Repository) {
	register(mux, db, resource[Project, nameInput]{
		table: "projects", collection: "/projects", item: "/projects/{id}",
		columns: "id, name", insert: "name", values: "$1", update: "name = $1", idParameter: "$2",
		validate: func(input nameInput) ([]any, string) {
			name := strings.TrimSpace(input.Name)
			if !projectName.MatchString(name) {
				return nil, "name must be a Compose project name of 1–100 characters"
			}
			return []any{name}, ""
		},
	})
	register(mux, db, resource[Service, nameInput]{
		table: "services", collection: "/projects/{parentID}/services", item: "/services/{id}",
		parentTable: "projects", parentColumn: "project_id",
		columns: "id, project_id, name", insert: "name, project_id", values: "$1, $2", update: "name = $1", idParameter: "$2",
		validate: func(input nameInput) ([]any, string) {
			name := strings.TrimSpace(input.Name)
			if !serviceName.MatchString(name) {
				return nil, "name must be a Compose service name of 1–100 characters"
			}
			return []any{name}, ""
		},
	})
	register(mux, db, resource[Incident, incidentInput]{
		table: "incidents", collection: "/services/{parentID}/incidents", item: "/incidents/{id}",
		parentTable: "services", parentColumn: "service_id",
		columns: "id, service_id, title, status, details", insert: "title, status, details, service_id",
		values: "$1, $2, $3, $4", update: "title = $1, status = $2, details = $3", idParameter: "$4",
		validate: validateIncident,
	})
	mux.HandleFunc("GET /services/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		id, ok := readID(w, r, "id")
		if !ok {
			return
		}

		ctx := r.Context()
		var project, service string
		err := db.QueryRow(ctx, `
			SELECT projects.name, services.name
			FROM services JOIN projects ON projects.id = services.project_id
			WHERE services.id = $1
		`, id).Scan(&project, &service)
		if err != nil {
			writeDatabaseError(w, err, false)
			return
		}

		result, err := eventRepository.ListForService(ctx, project, service)
		if err != nil {
			writeDatabaseError(w, err, false)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func validateIncident(input incidentInput) ([]any, string) {
	title := strings.TrimSpace(input.Title)
	if title == "" || utf8.RuneCountInString(title) > 200 || strings.ContainsRune(title, 0) {
		return nil, "title must contain 1–200 characters and no NUL characters"
	}
	switch input.Status {
	case "open", "investigating", "resolved":
	default:
		return nil, "status must be open, investigating, or resolved"
	}
	if utf8.RuneCountInString(input.Details) > 4000 || strings.ContainsRune(input.Details, 0) {
		return nil, "details must contain at most 4000 characters and no NUL characters"
	}
	return []any{title, input.Status, input.Details}, ""
}
