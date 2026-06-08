package api

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

func ok(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
}

func created(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": data})
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"status": "fail", "message": msg})
}

func badRequest(w http.ResponseWriter, msg string) { fail(w, http.StatusBadRequest, msg) }
func unauthorized(w http.ResponseWriter) { fail(w, http.StatusUnauthorized, "unauthorized") }

func Unauthorized(w http.ResponseWriter) { unauthorized(w) }
func notFound(w http.ResponseWriter)               { fail(w, http.StatusNotFound, "not found") }

func internalErr(w http.ResponseWriter, err error) {
	log.Error().Err(err).Msg("internal error")
	fail(w, http.StatusInternalServerError, "internal error")
}
