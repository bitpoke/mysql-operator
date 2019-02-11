package lager

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"go.uber.org/zap"
)

var _ lager.Logger = &ZapAdapter{}

// ZapAdapter is an adapter for lager log interface using zap logger
type ZapAdapter struct {
	sessionID string
	*zap.Logger
	origLogger *zap.Logger
	context    []zap.Field
}

func dataToFields(data ...lager.Data) (fields []zap.Field) {
	for _, d := range data {
		for k, v := range d {
			fields = append(fields, zap.Any(k, v))
		}
	}
	return fields
}

// NewZapAdapter creates a new ZapAdapter using the passed in zap.Logger
func NewZapAdapter(component string, zapLogger *zap.Logger) *ZapAdapter {
	logger := zapLogger.Named(component)
	return &ZapAdapter{
		sessionID:  component,
		Logger:     logger,
		origLogger: logger,
	}
}

// RegisterSink of a ZapAdapter does noting as sinnk is configured in the
// underlying zap logger
func (l *ZapAdapter) RegisterSink(_ lager.Sink) {}

// Session creates a new logger appending task to the current session
func (l *ZapAdapter) Session(task string, data ...lager.Data) lager.Logger {
	newSession := fmt.Sprintf("%s.%s", l.sessionID, task)
	logger := &ZapAdapter{
		sessionID:  newSession,
		origLogger: l.origLogger,
		Logger:     l.origLogger.Named(task),
		context:    append(l.context, dataToFields(data...)...),
	}
	return logger
}

// SessionName returns the current logging session name
func (l *ZapAdapter) SessionName() string {
	return l.sessionID
}

// WithData returns a new logger with specified data fields set
func (l *ZapAdapter) WithData(data lager.Data) lager.Logger {
	logger := &ZapAdapter{
		sessionID:  l.sessionID,
		origLogger: l.origLogger,
		Logger:     l.origLogger.With(dataToFields(data)...),
		context:    l.context,
	}
	return logger
}

// Debug logs a debug message
func (l *ZapAdapter) Debug(action string, data ...lager.Data) {
	l.Logger.Debug(action, append(l.context, dataToFields(data...)...)...)
}

// Info logs a informative message
func (l *ZapAdapter) Info(action string, data ...lager.Data) {
	l.Logger.Info(action, append(l.context, dataToFields(data...)...)...)
}

// Error logs an error message
func (l *ZapAdapter) Error(action string, err error, data ...lager.Data) {
	fields := append([]zap.Field{zap.Error(err)}, l.context...)
	fields = append(fields, dataToFields(data...)...)
	l.Logger.Error(action, fields...)
}

// Fatal logs an fatal error message
func (l *ZapAdapter) Fatal(action string, err error, data ...lager.Data) {
	fields := append([]zap.Field{zap.Error(err)}, l.context...)
	fields = append(fields, dataToFields(data...)...)
	l.Logger.Fatal(action, fields...)
}
