package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"
)

type Level int8

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelOff
)

func (lvl Level) String() string {
	switch {
	case lvl == LevelTrace:
		return "TRACE"
	case lvl == LevelDebug:
		return "DEBUG"
	case lvl == LevelInfo:
		return "INFO"
	case lvl == LevelWarn:
		return "WARN"
	case lvl == LevelError:
		return "ERROR"
	case lvl == LevelFatal:
		return "FATAL"
	default:
		return ""
	}
}

type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

func NewLogger(out io.Writer, minLevel Level) *Logger {
	return &Logger{out: out, minLevel: minLevel}
}

func (l *Logger) print(level Level, message string, properties map[string]string) (int, error) {

	if l.minLevel > level {
		return 0, nil
	}

	aux := struct {
		Level      string            `json:"level"`
		Time       string            `json:"time"`
		Message    string            `json:"message"`
		Properties map[string]string `json:"properties,omitempty"`
		Trace      string            `json:"trace,omitempty"`
	}{
		Level:      level.String(),
		Time:       time.Now().UTC().Format(time.RFC3339),
		Message:    message,
		Properties: properties,
	}

	// Include the stack of trace for entries at and above ERROR
	if level >= LevelError {
		aux.Trace = string(debug.Stack())
	}

	var auxBSON []byte
	auxBSON, err := json.Marshal(aux)
	if err != nil {
		auxBSON = []byte(LevelError.String() + ": unable to marshal log message:" + err.Error())
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return l.out.Write(append(auxBSON, '\n'))
}

func (l *Logger) PrintTrace(message string, properties map[string]string) {
	l.print(LevelTrace, message, properties)
}

func (l *Logger) PrintDebug(message string, properties map[string]string) {
	l.print(LevelDebug, message, properties)
}

func (l *Logger) PrintInfo(message string, properties map[string]string) {
	l.print(LevelInfo, message, properties)
}

func (l *Logger) PrintWarn(message string, properties map[string]string) {
	l.print(LevelWarn, message, properties)
}

func (l *Logger) PrintError(message string, properties map[string]string) {
	l.print(LevelError, message, properties)
}

func (l *Logger) PrintFatal(err error, properties map[string]string) {
	l.print(LevelFatal, err.Error(), properties)
	os.Exit(1) // For entries at the FATAL level, we also terminate the application.
}

func (l *Logger) Write(message []byte) (int, error) {
	return l.print(LevelError, string(message), nil)
}
