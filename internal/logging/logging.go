package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Configure writes the same structured events to the console and rotating file.
// No request bodies, query values, passwords or cookies are included in events.
func Configure(level, format, path string) (func() error, error) {
	var severity slog.Level
	if err := severity.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
		return nil, fmt.Errorf("invalid DASHBOARD_LOG_LEVEL: use debug, info, warn or error")
	}
	if format != "text" && format != "json" {
		return nil, fmt.Errorf("invalid DASHBOARD_LOG_FORMAT: use text or json")
	}
	output := io.Writer(os.Stdout)
	closeLog := func() error { return nil }
	if path != "-" {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
		// Check access now, instead of silently failing on the first log event.
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		writer := &lumberjack.Logger{Filename: path, MaxSize: 10, MaxBackups: 3, MaxAge: 7}
		output = io.MultiWriter(os.Stdout, writer)
		closeLog = writer.Close
	}
	options := &slog.HandlerOptions{Level: severity}
	var handler slog.Handler = slog.NewTextHandler(output, options)
	if format == "json" {
		handler = slog.NewJSONHandler(output, options)
	}
	slog.SetDefault(slog.New(handler))
	return closeLog, nil
}
