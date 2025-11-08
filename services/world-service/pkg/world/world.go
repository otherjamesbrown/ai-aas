package world

import (
	"encoding/json"
	"net/http"
	"time"
)

// Greeting contains the response payload for the world-service HTTP endpoint.
type Greeting struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Handler writes a JSON greeting with a server-generated timestamp.
func Handler(w http.ResponseWriter, r *http.Request) {
	payload := Greeting{
		Message:   "Hello, world-service!",
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(payload)
}
