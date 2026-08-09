package lib

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	sentryslog "github.com/getsentry/sentry-go/slog"
	"github.com/lmittmann/tint"
	"github.com/orsinium-labs/josh"
)

func NewLogHandler(config Config) slog.Handler {
	handler := newJSONHandler(config)
	if config.SentryDSN != "" {
		sentryHandler := newSentryHandler()
		handler = ChainHandlers(sentryHandler, handler)
	}
	return handler
}

// JSON logger on prod, a pretty colorful plaintext logger on dev.
func newJSONHandler(config Config) slog.Handler {
	opts := &slog.HandlerOptions{}
	if config.Debug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}
	var handler slog.Handler
	if config.Debug {
		handler = tint.NewTextHandler(os.Stdout, &tint.Options{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			err, isError := a.Value.Any().(error)
			if isError {
				aErr := tint.Err(err)
				aErr.Key = a.Key
				return aErr
			}
			return a
		}})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return handler
}

func newSentryHandler() slog.Handler {
	opt := sentryslog.Option{
		LogLevel:  []slog.Level{slog.LevelWarn},
		AddSource: true,
		AttrFromContext: []func(ctx context.Context) []slog.Attr{
			extractSentryAttrs,
		},
	}
	ctx := context.Background()
	return opt.NewSentryHandler(ctx)
}

// Extract from Context attributes for logging into Sentry.
func extractSentryAttrs(ctx context.Context) []slog.Attr {
	attrs := make([]slog.Attr, 0)
	jwt, jwtErr := josh.CGetSingleton[JWT](ctx)
	myID, myIDErr := josh.CGetSingleton[dbtypes.UserID](ctx)
	if jwtErr == nil && myIDErr == nil {
		strID := strconv.FormatInt(int64(myID), 10)
		attrs = append(attrs, slog.Group("user",
			slog.String("email", jwt.Email),
			slog.String("id", strID),
		))
	}
	return attrs
}

type chainHandler struct {
	head slog.Handler
	tail slog.Handler
}

func ChainHandlers(head, tail slog.Handler) slog.Handler {
	return &chainHandler{head, tail}
}

// Enabled implements [slog.Handler].
func (c *chainHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	if c.head.Enabled(ctx, lvl) {
		return true
	}
	return c.tail.Enabled(ctx, lvl)
}

// Handle implements [slog.Handler].
func (c *chainHandler) Handle(ctx context.Context, r slog.Record) error {
	var errHead error
	if c.head.Enabled(ctx, r.Level) {
		errHead = c.head.Handle(ctx, r)
	}
	var errTail error
	if c.tail.Enabled(ctx, r.Level) {
		errTail = c.tail.Handle(ctx, r)
	}
	if errHead != nil {
		return errHead
	}
	return errTail
}

// WithAttrs implements [slog.Handler].
func (c chainHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c.head = c.head.WithAttrs(attrs)
	c.tail = c.tail.WithAttrs(attrs)
	return &c
}

// WithGroup implements [slog.Handler].
func (c chainHandler) WithGroup(name string) slog.Handler {
	c.head = c.head.WithGroup(name)
	c.tail = c.tail.WithGroup(name)
	return &c
}
