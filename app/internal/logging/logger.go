package logging

import (
    "log"
    "os"
)

type Logger struct {
    info  *log.Logger
    warn  *log.Logger
    error *log.Logger
}

func New() *Logger {
    return &Logger{
        info:  log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
        warn:  log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime|log.Lshortfile),
        error: log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
    }
}

func (l *Logger) Info(v ...interface{})  { l.info.Println(v...) }
func (l *Logger) Warn(v ...interface{})  { l.warn.Println(v...) }
func (l *Logger) Error(v ...interface{}) { l.error.Println(v...) }

func (l *Logger) Infof(format string, v ...interface{})  { l.info.Printf(format, v...) }
func (l *Logger) Warnf(format string, v ...interface{})  { l.warn.Printf(format, v...) }
func (l *Logger) Errorf(format string, v ...interface{}) { l.error.Printf(format, v...) }


