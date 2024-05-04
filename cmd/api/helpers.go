package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kcharymyrat/greenlight/internal/validator"
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
	dataJSON, err := json.MarshalIndent(data, "", "  ")
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

func (app *application) readJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	maxBytes := 1_048_576 // 1 MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	err = decoder.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	res := qs.Get(key)

	if res == "" {
		return defaultValue
	}
	return res
}

func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	res := qs.Get(key)
	if res == "" {
		return defaultValue
	}

	return strings.Split(res, ",")
}

func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	res := qs.Get(key)
	if res == "" {
		return defaultValue
	}

	resInt, err := strconv.Atoi(res)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}
	return resInt
}
