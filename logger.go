package logging

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
)

// Logger — интерфейс логгера
type Logger interface {
	Debug(msg string, attrs ...slog.Attr)
	Info(msg string, attrs ...slog.Attr)
	Warn(msg string, attrs ...slog.Attr)
	Error(msg string, attrs ...slog.Attr)

	ErrorWithOp(msg string, err error, op string, attrs ...slog.Attr)

	// With возвращает логгер с предустановленными атрибутами,
	// которые будут добавляться к каждой записи.
	With(attrs ...slog.Attr) Logger
	// WithContext возвращает логгер, привязанный к контексту.
	// Контекст используется при логировании (важно для Sentry — hub/scope).
	WithContext(ctx context.Context) Logger

	// Вспомогательные методы для создания атрибутов
	Err(err error) slog.Attr
	Op(value string) slog.Attr
	Str(key, value string) slog.Attr
	Int(key string, value int) slog.Attr
	Float64(key string, value float64) slog.Attr
	Any(key string, value any) slog.Attr
}

var logger *slog.Logger

type Wrappedlogger struct {
	*slog.Logger
	ctx context.Context
}

var levelMap = map[int]slog.Level{
	5: slog.LevelDebug,
	4: slog.LevelInfo,
	3: slog.LevelWarn,
	2: slog.LevelError,
}

// NewLogger создаёт Logger из slog.Logger
func NewLogger(slogger *slog.Logger) *Wrappedlogger {
	return &Wrappedlogger{Logger: slogger, ctx: context.Background()}
}

func GetLogger() *Wrappedlogger {
	return NewLogger(logger)
}

func Initlogger(c *Config, sConfig *SentryConfig) *Wrappedlogger {

	var handler slog.Handler

	level, ok := levelMap[c.Level]
	if !ok {
		level = slog.LevelInfo
	}

	if !c.OutputInFile {
		// Вывод в stdout с текстовым форматом
		handler = tint.NewHandler(colorable.NewColorable(os.Stderr), &tint.Options{
			Level:      level,
			TimeFormat: "15:04:05",
			AddSource:  false,
			NoColor:    false,
		})
	} else {
		// Вывод в файл с JSON форматом
		fileName := "app.log"
		file, err := GetOutputLogFile(c.WorkingDir, c.Dir, fileName)
		if err != nil {
			log.Printf("Не удалось открыть файл логов %q, используется стандартный stderr\n%v", fileName, err)
			handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level:     level,
				AddSource: false,
			})
		} else {
			handler = slog.NewJSONHandler(file, &slog.HandlerOptions{
				Level:     level,
				AddSource: false,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.TimeKey {
						// Преобразуем время в строку с нужным форматом
						t := a.Value.Time()
						formatted := t.Format("2006-01-02 15:04:05")
						return slog.String(a.Key, formatted)
					}
					return a
				},
			})
		}
	}

	if sConfig.Use {

		sentrylevel, ok := levelMap[sConfig.Level]
		if !ok {
			sentrylevel = level
		}

		multiHandler := NewMultiHandler(handler, SentryHandler(sentrylevel))
		logger = slog.New(multiHandler)
	} else {
		logger = slog.New(handler)
	}

	return NewLogger(logger)
}

// With возвращает логгер с предустановленными атрибутами,
// которые будут добавляться к каждой последующей записи.
func (l *Wrappedlogger) With(attrs ...slog.Attr) Logger {
	return &Wrappedlogger{
		Logger: l.Logger.With(convertAttrsToAny(attrs)...),
		ctx:    l.context(),
	}
}

// WithContext возвращает логгер, привязанный к переданному контексту.
// Контекст передаётся в slog при логировании — это позволяет Sentry-хендлеру
// извлечь hub/scope из контекста.
func (l *Wrappedlogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Wrappedlogger{
		Logger: l.Logger,
		ctx:    ctx,
	}
}

// context возвращает привязанный контекст либо context.Background().
func (l *Wrappedlogger) context() context.Context {
	if l.ctx == nil {
		return context.Background()
	}
	return l.ctx
}

func (l *Wrappedlogger) Debug(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(l.context(), slog.LevelDebug, msg, attrs...)
}

func (l *Wrappedlogger) Info(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(l.context(), slog.LevelInfo, msg, attrs...)
}

func (l *Wrappedlogger) Warn(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(l.context(), slog.LevelWarn, msg, attrs...)
}

func (l *Wrappedlogger) Error(msg string, attrs ...slog.Attr) {
	l.Logger.LogAttrs(l.context(), slog.LevelError, msg, attrs...)
}

func (l *Wrappedlogger) ErrorWithOp(msg string, err error, op string, attrs ...slog.Attr) {
	args := make([]slog.Attr, 0, len(attrs)+2)
	if err != nil {
		args = append(args, l.Err(err))
	}
	if op != "" {
		args = append(args, l.Op(op))
	}
	args = append(args, attrs...)
	l.Logger.LogAttrs(l.context(), slog.LevelError, msg, args...)
}

func (l *Wrappedlogger) Err(err error) slog.Attr {
	return slog.Any("error", err)
}

func (l *Wrappedlogger) Op(value string) slog.Attr {
	return slog.Attr{
		Key:   "op",
		Value: slog.StringValue(value),
	}
}

func (l *Wrappedlogger) Str(key, value string) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.StringValue(value),
	}
}

func (l *Wrappedlogger) Int(key string, value int) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.IntValue(value),
	}
}

func (l *Wrappedlogger) Float64(key string, value float64) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.Float64Value(value),
	}
}

func (l *Wrappedlogger) Any(key string, value any) slog.Attr {
	return slog.Attr{
		Key:   key,
		Value: slog.AnyValue(value),
	}
}

func GetOutputLogFile(workingDir, logDir, fileName string) (*os.File, error) {

	fullLogDir := filepath.Join(workingDir, logDir)

	err := os.MkdirAll(fullLogDir, 0755)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/%s", fullLogDir, fileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	return file, err
}

func convertAttrsToAny(attrs []slog.Attr) []any {
	res := make([]any, len(attrs))
	for i, v := range attrs {
		res[i] = v
	}
	return res
}
