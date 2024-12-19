package utils

import (
	"os"
	"log"

	"github.com/fatih/color"
)

type Logger struct {
	*log.Logger
	level LogLevel
}

type LogLevel color.Attribute

const (
    INFO  = color.FgHiGreen
    DEBUG	= color.FgCyan
    WARN 	= color.FgYellow
    ERROR = color.FgHiRed
)

func New(prefix string, level LogLevel) *Logger {
	return &Logger{
		log.New(os.Stdout, prefix + " ", log.Ldate|log.Ltime|log.Lmicroseconds),
		level,
	}
}

func (l *Logger) Println(v ...interface{}) {
	l.Output(2, color.New(color.Attribute(l.level)).Sprint(v...))
}

func (l *Logger) Panicln(v ...interface{}) {
	l.Output(2, color.New(color.Attribute(l.level)).Sprint(v...))
	panic(v)
}

func (l *Logger) Printf(format string, v ...interface{}) {
	l.Output(2, color.New(color.Attribute(l.level)).Sprintf(format, v...))
}

func (l *Logger) Panicf(format string, v ...interface{}) {
	l.Output(2, color.New(color.Attribute(l.level)).Sprintf(format, v...))
	panic(v)
}

var (
	Info 	= New(color.New(color.BgHiGreen).Sprint(" INFO "), LogLevel(INFO))
	Warn	= New(color.New(color.BgYellow).Sprint(" WARN "), LogLevel(WARN))
	Debug	= New(color.New(color.BgCyan).Sprint(" DEBUG "), LogLevel(DEBUG))
	Error	= New(color.New(color.BgHiRed).Sprint(" ERROR "), LogLevel(ERROR))
)
