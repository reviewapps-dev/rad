package deploy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/caddy"
	"github.com/reviewapps-dev/rad/internal/callback"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/logging"
	"github.com/reviewapps-dev/rad/internal/port"
)

type Pipeline struct {
	steps  []Step
	cfg    *config.Config
	store  *app.Store
	ports  *port.Allocator
	caddy  *caddy.Manager
}

func NewPipeline(cfg *config.Config, store *app.Store, ports *port.Allocator, cm *caddy.Manager) *Pipeline {
	return &Pipeline{
		cfg:   cfg,
		store: store,
		ports: ports,
		caddy: cm,
	}
}

func (p *Pipeline) AddStep(s Step) {
	p.steps = append(p.steps, s)
}

func (p *Pipeline) Run(ctx context.Context, state *app.AppState, redeploy ...bool) error {
	// Set up log streaming — batch lines and send to callback URL
	var logStreamer *logBatcher
	if state.CallbackURL != "" {
		logsURL := strings.TrimSuffix(state.CallbackURL, "/status") + "/logs"
		logStreamer = newLogBatcher(state.AppID, logsURL, 5*time.Second)
		logStreamer.start()
	}

	logger := logging.NewDeployLogger(state.AppID, func(appID, line string) {
		p.store.AppendLog(appID, line)
		if logStreamer != nil {
			logStreamer.add(line)
		}
	})

	isRedeploy := len(redeploy) > 0 && redeploy[0]

	sctx := &StepContext{
		AppState:  state,
		Config:    p.cfg,
		Logger:    logger,
		Ports:     p.ports,
		Store:     p.store,
		Caddy:     p.caddy,
		EnvMap:    make(map[string]string),
		Processes: make(map[string]string),
		Redeploy:  isRedeploy,
	}

	if isRedeploy {
		logger.Log("starting redeploy pipeline for %s", state.AppID)
	} else {
		logger.Log("starting deploy pipeline for %s", state.AppID)
	}

	for _, step := range p.steps {
		select {
		case <-ctx.Done():
			return fmt.Errorf("deploy cancelled")
		default:
		}

		logger.Log("step: %s", step.Name())
		if err := step.Run(sctx); err != nil {
			logger.Log("step %s failed: %v", step.Name(), err)
			_ = p.store.UpdateStatus(state.AppID, app.StatusFailed, err.Error())

			// Run on_failure hooks (best-effort, don't fail on hook errors)
			if sctx.ReviewConfig != nil {
				logger.Log("running on_failure hooks")
				if hookErr := RunHooksFromConfig(sctx.ReviewConfig, HookOnFailure, sctx.RepoDir, state.RubyVersion, buildEnvSlice(sctx.EnvMap)); hookErr != nil {
					logger.Log("on_failure hook error (non-fatal): %v", hookErr)
				}
			}

			// Send failure callback to web app
			if state.CallbackURL != "" {
				logger.Log("sending failure callback to %s", state.CallbackURL)
				cb := callback.NewClient()
				cb.SendStatus(state.CallbackURL, callback.StatusPayload{
					AppID:  state.AppID,
					Status: string(app.StatusFailed),
					Error:  err.Error(),
				})
			}

			if logStreamer != nil {
				logStreamer.stop()
			}
			return fmt.Errorf("step %s: %w", step.Name(), err)
		}
	}

	logger.Log("deploy pipeline complete for %s", state.AppID)

	// Flush any remaining log lines
	if logStreamer != nil {
		logStreamer.stop()
	}

	return nil
}

// logBatcher collects log lines and sends them in batches to the callback logs URL.
type logBatcher struct {
	appID   string
	logsURL string
	client  *callback.Client
	lines   []string
	mu      sync.Mutex
	done    chan struct{}
	ticker  *time.Ticker
}

func newLogBatcher(appID, logsURL string, interval time.Duration) *logBatcher {
	return &logBatcher{
		appID:   appID,
		logsURL: logsURL,
		client:  callback.NewClient(),
		done:    make(chan struct{}),
		ticker:  time.NewTicker(interval),
	}
}

func (b *logBatcher) add(line string) {
	b.mu.Lock()
	b.lines = append(b.lines, line)
	b.mu.Unlock()
}

func (b *logBatcher) flush() {
	b.mu.Lock()
	if len(b.lines) == 0 {
		b.mu.Unlock()
		return
	}
	lines := b.lines
	b.lines = nil
	b.mu.Unlock()

	b.client.SendLogs(b.logsURL, callback.LogPayload{
		AppID: b.appID,
		Lines: lines,
	})
}

func (b *logBatcher) start() {
	go func() {
		for {
			select {
			case <-b.ticker.C:
				b.flush()
			case <-b.done:
				return
			}
		}
	}()
}

func (b *logBatcher) stop() {
	b.ticker.Stop()
	close(b.done)
	b.flush() // final flush
}
