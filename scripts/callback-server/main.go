package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/builds/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		body, _ := io.ReadAll(r.Body)

		var payload map[string]any
		json.Unmarshal(body, &payload)

		log.Printf("[callback] build %s status: %s", id, prettyJSON(payload))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	mux.HandleFunc("POST /api/v1/builds/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		body, _ := io.ReadAll(r.Body)

		var payload map[string]any
		json.Unmarshal(body, &payload)

		log.Printf("[logs] build %s: %d bytes", id, len(body))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	mux.HandleFunc("POST /api/v1/heartbeats", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var payload map[string]any
		json.Unmarshal(body, &payload)

		log.Printf("[heartbeat] %s", prettyJSON(payload))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	addr := ":3000"
	log.Printf("callback server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func prettyJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
