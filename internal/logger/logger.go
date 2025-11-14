package logger

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// LogLevel represents the logging level
type LogLevel string

const (
	DEBUG LogLevel = "DEBUG"
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	ERROR LogLevel = "ERROR"
)

// Init initializes the global logger with the specified log level
// logLevel should be one of: DEBUG, INFO, WARN, ERROR
// If invalid, defaults to INFO
func Init(logLevel string) {
	log = logrus.New()

	// Set output to stdout
	log.SetOutput(os.Stdout)

	// Set JSON formatter for structured logging
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		PrettyPrint:     false,
	})

	// Parse and set log level
	level, err := logrus.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		// Default to INFO if invalid level provided
		level = logrus.InfoLevel
		log.Warnf("Invalid log level '%s', defaulting to INFO", logLevel)
	}
	log.SetLevel(level)

	log.Infof("Logger initialized with level: %s", logLevel)
}

// GetLogger returns the global logger instance
func GetLogger() *logrus.Logger {
	if log == nil {
		// Initialize with default INFO level if not already initialized
		Init("INFO")
	}
	return log
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// WithField returns a logger entry with a single field
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields returns a logger entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}
