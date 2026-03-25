package config

import (
	"log/slog"
	"os"
	"time"
)

// Watcher polls a YAML config file for changes and calls onChange whenever
// the file is updated and parses successfully. Change detection uses mtime
// polling — no external dependencies are required.
type Watcher struct {
	path     string
	interval time.Duration
	onChange func(*Config)
	logger   *slog.Logger
	stop     chan struct{}
	lastMod  time.Time
}

// NewWatcher creates a watcher for the given config file path.
// onChange is called on the watcher's goroutine whenever the file changes
// and the new config parses without error. It must not block.
func NewWatcher(path string, interval time.Duration, logger *slog.Logger, onChange func(*Config)) *Watcher {
	return &Watcher{
		path:     path,
		interval: interval,
		onChange: onChange,
		logger:   logger,
		stop:     make(chan struct{}),
	}
}

// Start seeds the initial mtime and begins polling in a background goroutine.
// Call Stop to halt the watcher.
func (w *Watcher) Start() {
	if info, err := os.Stat(w.path); err == nil {
		w.lastMod = info.ModTime()
	}
	go w.loop()
}

// Stop halts the polling goroutine. Safe to call multiple times.
func (w *Watcher) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.check()
		case <-w.stop:
			return
		}
	}
}

func (w *Watcher) check() {
	info, err := os.Stat(w.path)
	if err != nil {
		w.logger.Warn("config watcher: stat failed", "path", w.path, "error", err)
		return
	}
	if !info.ModTime().After(w.lastMod) {
		return
	}
	cfg, err := Load(w.path)
	if err != nil {
		w.logger.Error("config watcher: reload failed, keeping current config",
			"path", w.path, "error", err)
		return
	}
	w.lastMod = info.ModTime()
	w.logger.Info("config reloaded", "path", w.path)
	w.onChange(cfg)
}
