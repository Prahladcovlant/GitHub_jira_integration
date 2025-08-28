package utils

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
}

func NewLogger() *Logger {
	return &Logger{
		infoLogger:  log.New(os.Stdout, "  INFO: ", log.LstdFlags),
		errorLogger: log.New(os.Stderr, " ERROR: ", log.LstdFlags),
	}
}

func (l *Logger) Info(message string) {
	l.infoLogger.Printf("%s", message)
}

func (l *Logger) Error(message string) {
	l.errorLogger.Printf("%s", message)
}

func (l *Logger) ProductionLog(eventType, details string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	l.Info(fmt.Sprintf(" PRODUCTION EVENT [%s] %s: %s", timestamp, eventType, details))
}
