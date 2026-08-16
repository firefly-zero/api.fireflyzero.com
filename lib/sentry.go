package lib

import (
	"context"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/orsinium-labs/josh"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

func withUserInfo(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		jwt := josh.Must(josh.GetSingleton[JWT](r))

		// https://docs.sentry.io/platforms/go/enriching-events/identify-user/
		hub := sentry.GetHubFromContext(r.Context()) //nolint:contextcheck
		if hub != nil {
			scope := hub.Scope()
			if scope != nil {
				scope.SetUser(sentry.User{
					ID:    jwt.SupabaseID.String(),
					Email: jwt.Email,
				})
			}
		}

		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(
			semconv.UserEmail(jwt.Email),
			semconv.UserID(jwt.SupabaseID.String()),
		)

		return h(r)
	}
}

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
