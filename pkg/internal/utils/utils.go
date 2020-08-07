package utils

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/paulcarlton-ww/go-utils/pkg/goutils"
)

// Log logs at given level.
func Log(logger logr.Logger, skip uint, level int, msg string, keysAndValues ...interface{}) {
	//line := fmt.Sprintf("%s %s", goutils.GetCaller(skip+3, true), msg)
	if level > 0 {
		keysAndValues = append(keysAndValues, "at", goutils.GetCaller(skip+3, true))
	}
	logger.V(level).Info(msg, keysAndValues...)
}

// LogError logs an error.
func LogError(logger logr.Logger, skip uint, err error, msg string, keysAndValues ...interface{}) {
	line := fmt.Sprintf("%s %s", goutils.GetCaller(skip+3, true), msg)
	logger.Error(err, line, keysAndValues...)
}
