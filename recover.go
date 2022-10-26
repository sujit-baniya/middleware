package middleware

import (
	"fmt"
	"github.com/sujit-baniya/framework/contracts/http"
	"github.com/sujit-baniya/framework/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ConfigRecover defines the config for middleware.
type ConfigRecover struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c http.Context) bool

	// EnableStackTrace enables handling stack trace
	//
	// Optional. Default: false
	EnableStackTrace bool

	Debug bool

	// StackTraceHandler defines a function to handle stack trace
	//
	// Optional. Default: defaultStackTraceHandler
	StackTraceHandler func(c http.Context, e interface{})

	ErrorHandler func(c http.Context, status int, e interface{}) error
}

var defaultStackTraceBufLen = 1 << 20

// ConfigRecoverDefault is the default config
var ConfigRecoverDefault = ConfigRecover{
	Next:              nil,
	EnableStackTrace:  false,
	StackTraceHandler: defaultStackTraceHandler,
	ErrorHandler:      defaultErrorHandler,
}

// Helper function to set default values
func configRecoverDefault(config ...ConfigRecover) ConfigRecover {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigRecoverDefault
	}

	// Override default config
	cfg := config[0]

	if cfg.EnableStackTrace && cfg.StackTraceHandler == nil {
		cfg.StackTraceHandler = defaultStackTraceHandler
	}
	if cfg.ErrorHandler == nil {
		cfg.ErrorHandler = defaultErrorHandler
	}

	return cfg
}

func getStackTrace(e interface{}) []byte {
	buf := make([]byte, defaultStackTraceBufLen)
	return buf[:runtime.Stack(buf, false)]
}

func getStackTraceWithoutPath(buf []byte, e interface{}) string {
	dir, _ := os.Getwd()
	baseDir := filepath.Dir(dir)
	stackTrace := fmt.Sprintf("panic: %v\n%s\n", e, buf)
	return strings.ReplaceAll(stackTrace, baseDir, "/root")
}

func defaultStackTraceHandler(c http.Context, e interface{}) {
	buf := getStackTrace(e)
	stackTrace := getStackTraceWithoutPath(buf, e)
	_, _ = os.Stderr.WriteString(stackTrace)
}

func defaultErrorHandler(c http.Context, status int, e interface{}) error {
	switch e := e.(type) {
	case []byte:
		return c.Status(status).String(string(e))
	case string:
		return c.Status(status).String(e)
	}
	return c.Status(status).Json(e)
}

// Recover creates a new middleware handler
func Recover(config ...ConfigRecover) http.HandlerFunc {
	// Set default config
	cfg := configRecoverDefault(config...)

	// Return new handler
	return func(c http.Context) (err error) {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Catch panics
		defer func() error {
			if r := recover(); r != nil {
				if cfg.EnableStackTrace {
					cfg.StackTraceHandler(c, r)
				}

				var ok bool
				if err, ok = r.(error); !ok {
					// Set error that will call the global error handler
					err = fmt.Errorf("%+v", r)
				}
				if err != nil {
					if cfg.EnableStackTrace && cfg.Debug {
						return cfg.ErrorHandler(c, utils.StatusInternalServerError, fmt.Sprintf("panic: %v\n%s\n", err, getStackTraceWithoutPath(getStackTrace(r), r)))
					}
					return cfg.ErrorHandler(c, utils.StatusInternalServerError, err.Error())
				}
			}
			return err
		}()
		// Return err if existed, else move to next handler
		return c.Next()
	}
}
