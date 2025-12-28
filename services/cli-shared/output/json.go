package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSONWriter provides JSON output formatting
type JSONWriter struct {
	writer  io.Writer
	pretty  bool
	encoder *json.Encoder
}

// NewJSONWriter creates a new JSON writer that outputs to stdout
func NewJSONWriter(pretty bool) *JSONWriter {
	return NewJSONWriterTo(os.Stdout, pretty)
}

// NewJSONWriterTo creates a new JSON writer with a custom output
func NewJSONWriterTo(w io.Writer, pretty bool) *JSONWriter {
	encoder := json.NewEncoder(w)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return &JSONWriter{
		writer:  w,
		pretty:  pretty,
		encoder: encoder,
	}
}

// Write outputs a value as JSON
func (j *JSONWriter) Write(v interface{}) error {
	return j.encoder.Encode(v)
}

// WriteString writes a JSON string
func (j *JSONWriter) WriteString(s string) error {
	return j.encoder.Encode(s)
}

// WriteError writes an error as JSON
func (j *JSONWriter) WriteError(err error) error {
	return j.encoder.Encode(map[string]string{
		"error": err.Error(),
	})
}

// WriteResult writes a result with status
func (j *JSONWriter) WriteResult(success bool, message string, data interface{}) error {
	result := map[string]interface{}{
		"success": success,
		"message": message,
	}
	if data != nil {
		result["data"] = data
	}
	return j.encoder.Encode(result)
}

// MarshalJSON marshals a value to JSON bytes
func MarshalJSON(v interface{}, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

// PrintJSON prints a value as JSON to stdout
// If only one argument is provided, defaults to pretty=true
func PrintJSON(v interface{}, prettyOpt ...bool) error {
	pretty := true // default to pretty
	if len(prettyOpt) > 0 {
		pretty = prettyOpt[0]
	}
	data, err := MarshalJSON(v, pretty)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// JSONResponse wraps an API response for JSON output
type JSONResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewSuccessResponse creates a successful JSON response
func NewSuccessResponse(message string, data interface{}) JSONResponse {
	return JSONResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}

// NewErrorResponse creates an error JSON response
func NewErrorResponse(err error) JSONResponse {
	return JSONResponse{
		Success: false,
		Error:   err.Error(),
	}
}
