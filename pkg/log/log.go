// SPDX-FileCopyrightText: © 2025 Nfrastack <code@nfrastack.com>
//
// SPDX-License-Identifier: BSD-3-Clause

package log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	LevelTrace   = "trace"
	LevelDebug   = "debug"
	LevelVerbose = "verbose"
	LevelInfo    = "info"
	LevelWarn    = "warn"
	LevelError   = "error"
)

type LogLevel int

const (
	LogLevelError   LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelVerbose
	LogLevelDebug
	LogLevelTrace
	LogLevelNone
)

var globalLogLevel LogLevel = LogLevelVerbose

func ParseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case LevelError:
		return LogLevelError
	case LevelWarn:
		return LogLevelWarn
	case LevelInfo:
		return LogLevelInfo
	case LevelVerbose:
		return LogLevelVerbose
	case LevelDebug:
		return LogLevelDebug
	case LevelTrace:
		return LogLevelTrace
	default:
		return LogLevelNone
	}
}

type Logger struct {
	prefix     string
	level      LogLevel
	isOverride bool
}

func NewLogger(prefix, logLevel string) *Logger {
	var level LogLevel
	var isOverride bool
	if logLevel == "" {
		level = globalLogLevel
		isOverride = false
	} else {
		level = ParseLogLevel(logLevel)
		if level == LogLevelNone {
			level = globalLogLevel
			isOverride = false
		} else {
			isOverride = true
		}
	}
	return &Logger{
		prefix:     prefix,
		level:      level,
		isOverride: isOverride,
	}
}

// For compatibility, alias NewScopedLogger to NewLogger
var NewScopedLogger = NewLogger

func updateGlobalLogLevel(levelStr string) {
	globalLogLevel = ParseLogLevel(levelStr)
	if globalLogLevel == LogLevelNone {
		globalLogLevel = LogLevelVerbose
	}
}

func (l *Logger) shouldLog(messageLevel LogLevel) bool {
	return messageLevel <= l.level
}

func (l *Logger) Debug(format string, args ...interface{}) {
	if l.shouldLog(LogLevelDebug) {
		message := fmt.Sprintf(format, args...)
		levelStr := "   DEBUG"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().debugLogger.Output(3, message)
	}
}

func (l *Logger) Trace(format string, args ...interface{}) {
	if l.shouldLog(LogLevelTrace) {
		message := fmt.Sprintf(format, args...)
		levelStr := "   TRACE"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().debugLogger.Output(3, message)
	}
}

func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.shouldLog(LogLevelVerbose) {
		message := fmt.Sprintf(format, args...)
		levelStr := " VERBOSE"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().infoLogger.Output(3, message)
	}
}

func (l *Logger) Info(format string, args ...interface{}) {
	if l.shouldLog(LogLevelInfo) {
		message := fmt.Sprintf(format, args...)
		levelStr := "    INFO"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().infoLogger.Output(3, message)
	}
}

func (l *Logger) Warn(format string, args ...interface{}) {
	if l.shouldLog(LogLevelWarn) {
		message := fmt.Sprintf(format, args...)
		levelStr := "    WARN"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().warnLogger.Output(3, message)
	}
}

func (l *Logger) Error(format string, args ...interface{}) {
	if l.shouldLog(LogLevelError) {
		message := fmt.Sprintf(format, args...)
		levelStr := "   ERROR"
		if l.prefix != "" {
			message = fmt.Sprintf("%s %s", l.prefix, message)
		}
		if GetLogger().showTimestamps {
			timestamp := time.Now().Format("2006-01-02 15:04:05")
			message = fmt.Sprintf("%s %s %s", timestamp, levelStr, message)
		} else {
			message = fmt.Sprintf("%s %s", levelStr, message)
		}
		GetLogger().errorLogger.Output(3, message)
	}
}

// Logger provides logging functionality for the application
type ApplicationLogger struct {
	debugLogger   *log.Logger
	infoLogger    *log.Logger
	warnLogger    *log.Logger
	errorLogger   *log.Logger
	showTimestamps bool
	mu            sync.Mutex
}

var loggerInstance *ApplicationLogger
var once sync.Once

func GetLogger() *ApplicationLogger {
	once.Do(func() {
		loggerInstance = &ApplicationLogger{
			debugLogger:   log.New(os.Stdout, "", 0),
			infoLogger:    log.New(os.Stdout, "", 0),
			warnLogger:    log.New(os.Stdout, "", 0),
			errorLogger:   log.New(os.Stderr, "", 0),
			showTimestamps: true,
		}
	})
	return loggerInstance
}

func (l *ApplicationLogger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.debugLogger.SetOutput(w)
	l.infoLogger.SetOutput(w)
	l.warnLogger.SetOutput(w)
	l.errorLogger.SetOutput(w)
}

func (l *ApplicationLogger) SetShowTimestamps(show bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.showTimestamps = show
}
