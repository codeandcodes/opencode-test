package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleSSE streams runner progress events as server-sent events.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events, unsubscribe := s.cfg.Runner.Subscribe()
	defer unsubscribe()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case e := <-events:
			payload, err := json.Marshal(e)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", payload)
			fl.Flush()
		}
	}
}
