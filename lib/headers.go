package lib

import (
	"time"

	"github.com/orsinium-labs/josh"
	"github.com/orsinium-labs/josh/statuses"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

const contentType = "application/vnd.api+json"

// Validate HTTP headers.
func withHeaders(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		accept := r.Header.Get("Accept")
		if accept == "" {
			return josh.Resp{
				Status: statuses.UnsupportedMediaType,
				Errors: []josh.Error{{Detail: "Accept header is missing"}},
			}
		}
		if accept != contentType {
			return josh.Resp{
				Status: statuses.UnsupportedMediaType,
				Errors: []josh.Error{{Detail: "invalid Accept header"}},
			}
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" {
			return josh.Resp{
				Status: statuses.UnsupportedMediaType,
				Errors: []josh.Error{{Detail: "Content-Type header is missing"}},
			}
		}
		if ct != contentType {
			return josh.Resp{
				Status: statuses.UnsupportedMediaType,
				Errors: []josh.Error{{Detail: "invalid Content-Type header"}},
			}
		}

		apiVersion := r.Header.Get("X-Api-Version")
		if apiVersion == "" {
			return josh.BadRequest(josh.Error{Detail: "X-Api-Version header is missing"})
		}
		versionDate, err := time.Parse(time.DateOnly, apiVersion)
		if err != nil {
			return josh.BadRequest(josh.Error{Detail: "invalid X-Api-Version"})
		}
		if versionDate.Year() < 2025 {
			return josh.BadRequest(josh.Error{Detail: "unsupported X-Api-Version"})
		}

		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(
			semconv.ServiceVersion(apiVersion),
		)

		return h(r)
	}
}
