package layers

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Logger is a global logger instance for the application.
var Logger *log.Logger

// InitLogger initializes the global logger.
func InitLogger(logFilePath string) error {
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	Logger = log.New(file, "OSI-Tester: ", log.Ldate|log.Ltime|log.Lshortfile)
	return nil
}

// LogInfo logs an informational message.
func LogInfo(message string) {
	if Logger != nil {
		Logger.Println("INFO: " + message)
	} else {
		fmt.Println("INFO: " + message) // Fallback to console if logger is not initialized
	}
}

// LogError logs an error message.
func LogError(err error) {
	if Logger != nil {
		Logger.Println("ERROR: " + err.Error())
	} else {
		fmt.Println("ERROR: " + err.Error()) // Fallback to console if logger is not initialized
	}
}

// MeasureExecutionTime measures the execution time of a function and logs it.
func MeasureExecutionTime(label string, f func()) {
	start := time.Now()
	f()
	duration := time.Since(start)
	LogInfo(fmt.Sprintf("%s completed in %v", label, duration))
}

// FileExists checks if a file exists at the given path.
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// FormatDuration formats a time.Duration into a human-readable string.
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}

// GetTimestamp returns the current timestamp in a readable format.
func GetTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
