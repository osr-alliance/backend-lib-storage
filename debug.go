package storage

import (
	"context"

	"github.com/sirupsen/logrus"
)

var debug *logger

func d(s string, args ...interface{}) {
	debug.debug(s, args...)
}

type logger struct {
	entry           *logrus.Entry
	debuggerEnabled bool
}

func (l *logger) debug(s string, args ...interface{}) {
	if l.debuggerEnabled && l.entry != nil {
		l.entry.Debugf(s, args...)
	}
}

func (l *logger) WithField(key string, value interface{}) {
	if l.debuggerEnabled && l.entry != nil {
		l.entry = l.entry.WithField(key, value)
	}
}

func (l *logger) WithFields(fields logrus.Fields) {
	if l.debuggerEnabled && l.entry != nil {
		l.entry = l.entry.WithFields(fields)
	}
}

func (l *logger) clean() {
	l.entry = nil
}

func (l *logger) init(ctx context.Context) {
	if l.debuggerEnabled {
		l.entry = logrus.WithContext(ctx)
	}
}
