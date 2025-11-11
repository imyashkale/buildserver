package services

import (
	"sync"
	"time"

	"github.com/imyashkale/buildserver/internal/models"
)

const (
	LevelInfo    = "info"
	LevelWarning = "warning"
	LevelError   = "error"

	LogSizeLimit = 400 * 1024 // 400KB limit
)

// BuildLogger handles structured logging for build operations
type BuildLogger struct {
	logs  []models.BuildLogEntry
	mu    sync.Mutex
	stage string
}

// NewBuildLogger creates a new build logger for a specific stage
func NewBuildLogger() *BuildLogger {
	return &BuildLogger{
		logs: make([]models.BuildLogEntry, 0),
	}
}

// LogInfo logs an info level message
func (bl *BuildLogger) LogInfo(stage, message string) {
	bl.log(stage, LevelInfo, message)
}

// LogWarning logs a warning level message
func (bl *BuildLogger) LogWarning(stage, message string) {
	bl.log(stage, LevelWarning, message)
}

// LogError logs an error level message
func (bl *BuildLogger) LogError(stage, message string) {
	bl.log(stage, LevelError, message)
}

// log is the internal method that adds a log entry
func (bl *BuildLogger) log(stage, level, message string) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	entry := models.BuildLogEntry{
		Timestamp: time.Now(),
		Stage:     stage,
		Level:     level,
		Message:   message,
	}

	bl.logs = append(bl.logs, entry)
}

// GetLogs returns all logged entries
func (bl *BuildLogger) GetLogs() []models.BuildLogEntry {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	// Return a copy to prevent external modifications
	logsCopy := make([]models.BuildLogEntry, len(bl.logs))
	copy(logsCopy, bl.logs)
	return logsCopy
}

// GetLogsWithSizeLimit returns logs but truncates if they exceed the size limit
// This prevents DynamoDB items from exceeding their size limits
func (bl *BuildLogger) GetLogsWithSizeLimit() []models.BuildLogEntry {
	logs := bl.GetLogs()

	// Estimate size (rough calculation)
	var totalSize int
	var result []models.BuildLogEntry

	for _, log := range logs {
		// Rough size estimation: timestamp (25) + stage (50) + level (10) + message (len) + overhead (50)
		entrySize := 135 + len(log.Message)
		if totalSize+entrySize > LogSizeLimit {
			// Add a truncation notice
			result = append(result, models.BuildLogEntry{
				Timestamp: time.Now(),
				Stage:     "system",
				Level:     LevelWarning,
				Message:   "Log output exceeded size limit. Older logs truncated.",
			})
			break
		}
		result = append(result, log)
		totalSize += entrySize
	}

	return result
}

// Clear clears all logs
func (bl *BuildLogger) Clear() {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.logs = make([]models.BuildLogEntry, 0)
}
