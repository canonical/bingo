package main

import (
	"log/slog"
	"testing"
)

func TestNewLogger_levels(t *testing.T) {
	tests := []struct {
		name string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"info", slog.LevelInfo},
		{"unknown-defaults-to-info", slog.LevelInfo},
		{"", slog.LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := newLogger(tt.name)
			if logger == nil {
				t.Fatal("newLogger() = nil")
			}
			if !logger.Enabled(t.Context(), tt.want) {
				t.Errorf("newLogger(%q) not enabled at level %v", tt.name, tt.want)
			}
			// The next level down (Debug->doesn't apply; else check one level below tt.want is disabled)
			if tt.want != slog.LevelDebug {
				below := tt.want - 1
				if logger.Enabled(t.Context(), below) {
					t.Errorf("newLogger(%q) unexpectedly enabled at level %v (below configured level %v)", tt.name, below, tt.want)
				}
			}
		})
	}
}
