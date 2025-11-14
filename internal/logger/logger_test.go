package logger

import (
	"testing"

	"github.com/sirupsen/logrus"
)

// TestLoggerInitialization tests that logger can be initialized with different log levels
func TestLoggerInitialization(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		want    logrus.Level
		wantErr bool
	}{
		{
			name:    "Valid DEBUG level",
			level:   "DEBUG",
			want:    logrus.DebugLevel,
			wantErr: false,
		},
		{
			name:    "Valid INFO level",
			level:   "INFO",
			want:    logrus.InfoLevel,
			wantErr: false,
		},
		{
			name:    "Valid WARN level",
			level:   "WARN",
			want:    logrus.WarnLevel,
			wantErr: false,
		},
		{
			name:    "Valid ERROR level",
			level:   "ERROR",
			want:    logrus.ErrorLevel,
			wantErr: false,
		},
		{
			name:    "Invalid level defaults to INFO",
			level:   "INVALID",
			want:    logrus.InfoLevel,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Init(tt.level)
			if GetLogger().Level != tt.want {
				t.Errorf("Expected level %v, got %v", tt.want, GetLogger().Level)
			}
		})
	}
}

// TestLoggerMethods tests that logger methods work correctly
func TestLoggerMethods(t *testing.T) {
	Init("DEBUG")

	tests := []struct {
		name     string
		method   string
		testFunc func()
	}{
		{
			name:   "Debug method",
			method: "Debug",
			testFunc: func() {
				Debug("test debug message")
			},
		},
		{
			name:   "Debugf method",
			method: "Debugf",
			testFunc: func() {
				Debugf("test debug format %s", "message")
			},
		},
		{
			name:   "Info method",
			method: "Info",
			testFunc: func() {
				Info("test info message")
			},
		},
		{
			name:   "Infof method",
			method: "Infof",
			testFunc: func() {
				Infof("test info format %s", "message")
			},
		},
		{
			name:   "Warn method",
			method: "Warn",
			testFunc: func() {
				Warn("test warn message")
			},
		},
		{
			name:   "Warnf method",
			method: "Warnf",
			testFunc: func() {
				Warnf("test warn format %s", "message")
			},
		},
		{
			name:   "Error method",
			method: "Error",
			testFunc: func() {
				Error("test error message")
			},
		},
		{
			name:   "Errorf method",
			method: "Errorf",
			testFunc: func() {
				Errorf("test error format %s", "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This just ensures the methods don't panic
			tt.testFunc()
		})
	}
}

// TestLoggerWithFields tests that logger can add contextual fields
func TestLoggerWithFields(t *testing.T) {
	Init("INFO")

	t.Run("WithField method", func(t *testing.T) {
		entry := WithField("user_id", "12345")
		if entry == nil {
			t.Errorf("WithField should return a non-nil entry")
		}
	})

	t.Run("WithFields method", func(t *testing.T) {
		entry := WithFields(logrus.Fields{
			"user_id":  "12345",
			"action":   "create",
			"resource": "mcp_server",
		})
		if entry == nil {
			t.Errorf("WithFields should return a non-nil entry")
		}
	})
}
