package catalog

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQL identifiers and fragments are supplied by the route declarations.
// Values supplied by API clients are always bound as PostgreSQL parameters.
type catalogRow interface {
	resourceID() int64
}

type resource[T catalogRow, Input any] struct {
	table, collection, item   string
	parentTable, parentColumn string
	columns, insert, values   string
	update, idParameter       string
	validate                  func(Input) ([]any, string)
}

func register[T catalogRow, Input any](mux *http.ServeMux, db *pgxpool.Pool, cfg resource[T, Input]) {
	selectRows := "SELECT " + cfg.columns + " FROM " + cfg.table
	listRows := selectRows + " ORDER BY id"
	var parentExists string
	if cfg.parentTable != "" {
		listRows = selectRows + " WHERE " + cfg.parentColumn + " = $1 ORDER BY id"
		parentExists = "SELECT EXISTS (SELECT 1 FROM " + cfg.parentTable + " WHERE id = $1)"
	}
	getRow := selectRows + " WHERE id = $1"
	insertRow := "INSERT INTO " + cfg.table + " (" + cfg.insert + ") VALUES (" + cfg.values + ") RETURNING " + cfg.columns
	updateRow := "UPDATE " + cfg.table + " SET " + cfg.update + " WHERE id = " + cfg.idParameter + " RETURNING " + cfg.columns
	deleteRow := "DELETE FROM " + cfg.table + " WHERE id = $1"

	mux.HandleFunc("GET "+cfg.collection, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		query := db.Query
		var args []any
		if parentExists != "" {
			parentID, ok := readID(w, r, "parentID")
			if !ok {
				return
			}
			tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
			if err != nil {
				writeDatabaseError(w, err, false)
				return
			}
			defer tx.Rollback(ctx)

			var exists bool
			if err := tx.QueryRow(ctx, parentExists, parentID).Scan(&exists); err != nil {
				writeDatabaseError(w, err, false)
				return
			}
			if !exists {
				writeError(w, http.StatusNotFound, "parent resource not found")
				return
			}
			query = tx.Query
			args = []any{parentID}
		}
		rows, err := query(ctx, listRows, args...)
		if err != nil {
			writeDatabaseError(w, err, false)
			return
		}
		result, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
		if err != nil {
			writeDatabaseError(w, err, false)
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET "+cfg.item, func(w http.ResponseWriter, r *http.Request) {
		id, ok := readID(w, r, "id")
		if ok {
			writeRow[T](w, http.StatusOK, db, r, "", getRow, id)
		}
	})

	mux.HandleFunc("POST "+cfg.collection, func(w http.ResponseWriter, r *http.Request) {
		var parentID int64
		if cfg.parentTable != "" {
			var ok bool
			parentID, ok = readID(w, r, "parentID")
			if !ok {
				return
			}
		}
		args, ok := readInput(w, r, cfg.validate)
		if !ok {
			return
		}
		if cfg.parentTable != "" {
			args = append(args, parentID)
		}
		writeRow[T](w, http.StatusCreated, db, r, strings.TrimSuffix(cfg.item, "{id}"), insertRow, args...)
	})

	mux.HandleFunc("PUT "+cfg.item, func(w http.ResponseWriter, r *http.Request) {
		id, ok := readID(w, r, "id")
		if !ok {
			return
		}
		args, ok := readInput(w, r, cfg.validate)
		if ok {
			writeRow[T](w, http.StatusOK, db, r, "", updateRow, append(args, id)...)
		}
	})

	mux.HandleFunc("DELETE "+cfg.item, func(w http.ResponseWriter, r *http.Request) {
		id, ok := readID(w, r, "id")
		if !ok {
			return
		}
		result, err := db.Exec(r.Context(), deleteRow, id)
		if err != nil {
			writeDatabaseError(w, err, true)
			return
		}
		if result.RowsAffected() == 0 {
			writeError(w, http.StatusNotFound, "resource not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func readID(w http.ResponseWriter, r *http.Request, param string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(param), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "ID must be a positive 64-bit integer")
		return 0, false
	}
	return id, true
}

func readInput[Input any](w http.ResponseWriter, r *http.Request, validate func(Input) ([]any, string)) ([]any, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return nil, false
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	var input *Input
	err = decoder.Decode(&input)
	if err == nil {
		if trailing := decoder.Decode(new(json.RawMessage)); trailing != io.EOF {
			err = trailing
			if trailing == nil {
				err = errors.New("multiple JSON values")
			}
		}
	}
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "request body exceeds 64 KiB")
		return nil, false
	case err != nil || input == nil:
		writeError(w, http.StatusBadRequest, "expected one JSON object with valid fields")
		return nil, false
	}

	args, message := validate(*input)
	if message != "" {
		writeError(w, http.StatusBadRequest, message)
		return nil, false
	}
	return args, true
}

func writeRow[T catalogRow](w http.ResponseWriter, status int, db *pgxpool.Pool, r *http.Request, location, query string, args ...any) {
	rows, err := db.Query(r.Context(), query, args...)
	if err != nil {
		writeDatabaseError(w, err, false)
		return
	}
	result, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if err != nil {
		writeDatabaseError(w, err, false)
		return
	}
	if location != "" {
		w.Header().Set("Location", location+strconv.FormatInt(result.resourceID(), 10))
	}
	writeJSON(w, status, result)
}

func writeDatabaseError(w http.ResponseWriter, err error, deleting bool) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		writeError(w, http.StatusConflict, "name already exists")
	case pgErr != nil && pgErr.Code == "23503" && deleting:
		writeError(w, http.StatusConflict, "delete child resources first")
	case pgErr != nil && pgErr.Code == "23503":
		writeError(w, http.StatusNotFound, "parent resource not found")
	default:
		log.Printf("catalog database error: %v", err)
		writeError(w, http.StatusInternalServerError, "database request failed")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
