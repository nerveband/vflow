package output

import (
	"encoding/json"
	"io"

	verrors "github.com/nerveband/vflow/internal/errors"
)

const (
	ResponseSchemaVersion = "vflow-response/v1"
	ErrorSchemaVersion    = "vflow-error/v1"
)

type Response struct {
	OK            bool        `json:"ok"`
	SchemaVersion string      `json:"schema_version"`
	Command       string      `json:"command"`
	Data          interface{} `json:"data"`
}

type ErrorResponse struct {
	OK            bool           `json:"ok"`
	SchemaVersion string         `json:"schema_version"`
	Error         *verrors.Error `json:"error"`
}

func Envelope(command string, data interface{}) Response {
	return Response{
		OK:            true,
		SchemaVersion: ResponseSchemaVersion,
		Command:       command,
		Data:          data,
	}
}

func ErrorEnvelope(err *verrors.Error) ErrorResponse {
	return ErrorResponse{
		OK:            false,
		SchemaVersion: ErrorSchemaVersion,
		Error:         err,
	}
}

func WriteJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
