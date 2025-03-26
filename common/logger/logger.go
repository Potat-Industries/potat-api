// Package logger provides utility functions for logging and other common tasks.
package logger

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fatih/color"
)

// Logger is a custom logger with log levels and colors.
type Logger struct {
	*log.Logger
	level string
	text  color.Attribute
	tag   color.Attribute
}

//nolint:gochecknoglobals,revive // skibidi toilet
var (
	Info  = createLogger(color.FgHiGreen, color.BgHiGreen, "INFO")
	Debug = createLogger(color.FgCyan, color.BgCyan, "DEBUG")
	Warn  = createLogger(color.FgYellow, color.BgYellow, "WARN")
	Error = createLogger(color.FgRed, color.BgRed, "ERROR")
)

func init() {
	color.NoColor = false
}

func createLogger(text, tag color.Attribute, level string) *Logger {
	return &Logger{
		Logger: log.New(os.Stdout, "", 0),
		level:  level,
		text:   text,
		tag:    tag,
	}
}

func getLocalTime() string {
	loc, _ := time.LoadLocation("America/Anchorage")
	now := time.Now().In(loc)

	return now.Format("01/02/2006 15:04:05") + fmt.Sprintf(".%06d", now.Nanosecond()/1000)
}

func (l *Logger) toOutput(format *string, v ...any) {
	var message string

	timeString := getLocalTime()
	if format != nil {
		message = color.New(l.text).Sprintf(*format, v...)
	} else {
		message = color.New(l.text).Sprint(v...)
	}

	tag := color.New(l.tag).Sprintf(" %s ", l.level)
	output := fmt.Sprintf("API %s %s %s", timeString, tag, message)

	_ = l.Output(2, output)
}

// Println prints a message with a newline.
func (l *Logger) Println(v ...any) {
	l.toOutput(nil, v...)
}

// Printf prints a formatted message.
func (l *Logger) Printf(format string, v ...any) {
	l.toOutput(&format, v...)
}
