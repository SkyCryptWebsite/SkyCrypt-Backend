package forensics

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger *zap.Logger
var SugaredLogger *zap.SugaredLogger

// Log rotation config — keeps ~3000MB total max on disk:
//   - MaxSize:    500 MB per file before rotating
//   - MaxBackups: 5 old files kept (+ current = 6 files max)
//   - MaxAge:     7 days, then auto-deleted
//   - Compress:   old rotated files are gzipped
func InitLogger() {
	_ = os.MkdirAll("logs", 0755)

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	jsonEncoder := zapcore.NewJSONEncoder(encoderCfg)

	// Rotating file writer for all logs
	appLogWriter := &lumberjack.Logger{
		Filename:   "logs/app.log",
		MaxSize:    500,  // megabytes per file
		MaxBackups: 5,    // keep 5 old files
		MaxAge:     7,    // days
		Compress:   true, // gzip rotated files
	}

	// Rotating file writer for error-only logs
	errorLogWriter := &lumberjack.Logger{
		Filename:   "logs/error.log",
		MaxSize:    500,
		MaxBackups: 5,
		MaxAge:     7,
		Compress:   true,
	}

	cores := []zapcore.Core{
		zapcore.NewCore(jsonEncoder, zapcore.AddSync(appLogWriter), zapcore.DebugLevel),
		zapcore.NewCore(jsonEncoder, zapcore.AddSync(errorLogWriter), zapcore.WarnLevel),
	}

	if os.Getenv("LOG_STDOUT") == "1" {
		cores = append(cores, zapcore.NewCore(jsonEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel))
	}

	core := zapcore.NewTee(cores...)

	Logger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	SugaredLogger = Logger.Sugar()

	Logger.Info("forensic_logger_initialized",
		zap.String("go_version", runtime.Version()),
		zap.Int("num_cpu", runtime.NumCPU()),
		zap.Int("gomaxprocs", runtime.GOMAXPROCS(0)),
		zap.String("hostname", getHostname()),
		zap.Int("pid", os.Getpid()),
		zap.Int("log_max_size_mb", 50),
		zap.Int("log_max_backups", 5),
		zap.Int("log_max_age_days", 7),
		zap.Bool("log_compress", true),
	)
}

func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// PrintError prints a message to stderr in red AND logs it as an error in zap.
func PrintError(msg string, fields ...zap.Field) {
	fmt.Fprintf(os.Stderr, "\033[31m[ERROR] %s\033[0m\n", msg)
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// PrintWarn prints a message to stderr in yellow AND logs it as a warning in zap.
func PrintWarn(msg string, fields ...zap.Field) {
	fmt.Fprintf(os.Stderr, "\033[33m[WARN] %s\033[0m\n", msg)
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// PrintFatal prints a message to stderr in red AND logs it as fatal in zap (which calls os.Exit).
func PrintFatal(msg string, fields ...zap.Field) {
	fmt.Fprintf(os.Stderr, "\033[31m[FATAL] %s\033[0m\n", msg)
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	}
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// PerfLogger tracks operation durations and flags slow operations.
type PerfLogger struct {
	logger *zap.Logger
}

func NewPerfLogger() *PerfLogger {
	return &PerfLogger{logger: Logger}
}

// TrackOperation returns a stop function that logs duration when called.
func (p *PerfLogger) TrackOperation(operation string) func() {
	start := time.Now()

	return func() {
		duration := time.Since(start)

		p.logger.Info("operation_completed",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
			zap.Int64("duration_ms", duration.Milliseconds()),
			zap.Int64("duration_us", duration.Microseconds()),
		)

		if duration > 100*time.Millisecond {
			p.logger.Warn("slow_operation_detected",
				zap.String("operation", operation),
				zap.Duration("duration", duration),
				zap.String("threshold", "100ms"),
			)
		}

		if duration > 1*time.Second {
			p.logger.Error("critical_slow_operation",
				zap.String("operation", operation),
				zap.Duration("duration", duration),
				zap.String("threshold", "1s"),
			)
		}
	}
}

// TrackFunc is a convenience for tracking a named function call.
func TrackFunc(name string) func() {
	return NewPerfLogger().TrackOperation(name)
}

// GetAllocBytes returns current heap allocation in bytes.
func GetAllocBytes() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
