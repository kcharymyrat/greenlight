package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type envelope map[string]interface{}

func (app *application) readIdParam(r *http.Request) (int64, error) {
	idStr := chi.URLParamFromCtx(r.Context(), "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid 'id' parameter")
	}
	return id, err
}

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	dataJSON = append(dataJSON, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(dataJSON)

	return nil
}
