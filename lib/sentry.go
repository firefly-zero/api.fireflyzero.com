package lib

import (
	"context"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/orsinium-labs/josh"
)

// Sentry go/http middleware adapted for use with josh.
//
// Source:
// https://github.com/getsentry/sentry-go/blob/master/http/sentryhttp.go
func Sentry(handler josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		ctx := r.Context()
		hub := sentry.GetHubFromContext(r.Context())
		if hub == nil {
			hub = sentry.CurrentHub().Clone()
			ctx = sentry.SetHubOnContext(ctx, hub)
		}

		if client := hub.Client(); client != nil {
			client.SetSDKIdentifier("sentry.go.http")
		}

		options := []sentry.SpanOption{
			sentry.ContinueTrace(hub, r.Header.Get(sentry.SentryTraceHeader), r.Header.Get(sentry.SentryBaggageHeader)),
			sentry.WithOpName("http.server"),
			sentry.WithTransactionSource(sentry.SourceURL),
			sentry.WithSpanOrigin(sentry.SpanOriginStdLib),
		}

		transaction := sentry.StartTransaction(ctx, r.Pattern, options...)
		transaction.SetData("http.request.method", r.Method)

		defer transaction.Finish()
		hub.Scope().SetRequest(r)
		r = r.WithContext(transaction.Context()) //nolint:contextcheck
		defer recoverWithSentry(hub, r)

		return handler(r)
	}
}

func recoverWithSentry(hub *sentry.Hub, r *http.Request) {
	err := recover()
	if err != nil {
		ctx := context.WithValue(r.Context(), sentry.RequestContextKey, r)
		hub.RecoverWithContext(ctx, err)
		panic(err)
	}
}
