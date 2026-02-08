package server

import (
	"context"
	"log"
	"net/http"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/logstream"
	"nhooyr.io/websocket"
)

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow any origin (token auth is sufficient)
	})
	if err != nil {
		log.Printf("ws: accept failed for %s: %v", appID, err)
		return
	}
	defer conn.CloseNow()

	ctx := conn.CloseRead(r.Context())

	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "build"
	}

	switch logType {
	case "build":
		s.streamBuildLogs(ctx, conn, appID, state)
	case "runtime":
		processName := r.URL.Query().Get("process")
		if processName == "" {
			processName = "web"
		}
		s.streamRuntimeLogs(ctx, conn, appID, processName)
	default:
		conn.Close(websocket.StatusPolicyViolation, "invalid type: use 'build' or 'runtime'")
		return
	}

	conn.Close(websocket.StatusNormalClosure, "done")
}

func (s *Server) streamBuildLogs(ctx context.Context, conn *websocket.Conn, appID string, state *app.AppState) {
	// Subscribe first so we don't miss lines published between reading backlog and subscribing
	ch, unsub := s.hub.Subscribe(appID)
	defer unsub()

	// Send existing build log as backlog
	for _, line := range state.BuildLog {
		if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
			return
		}
	}

	// If the deploy is already done (not queued/building/cloning/starting), close immediately
	if state.Status == app.StatusRunning || state.Status == app.StatusFailed || state.Status == app.StatusStopped {
		return
	}

	// Stream new lines from the hub until deploy finishes or client disconnects
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				// Channel closed — deploy finished
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
				return
			}
		}
	}
}

func (s *Server) streamRuntimeLogs(ctx context.Context, conn *websocket.Conn, appID, processName string) {
	logPath := s.processLogPath(appID, processName)
	tailer := logstream.NewTailer(logPath, 100)
	ch := tailer.Start(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
				return
			}
		}
	}
}
