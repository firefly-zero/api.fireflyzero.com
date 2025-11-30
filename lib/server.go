package lib

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
	"github.com/orsinium-labs/josh/middlewares"
)

type Clock func() time.Time

type Database interface {
	db.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Server struct {
	Logger  *slog.Logger
	Config  Config
	Queries *db.Queries
	DB      Database
	Clock   Clock
}

func wrap(s Server, h josh.Handler) http.HandlerFunc {
	// h = withMe(h)
	h = middlewares.Auth(authValidator(s.Config.AuthSecret), h)
	return wrapNoAuth(s, h)
}

func wrapNoAuth(s Server, h josh.Handler) http.HandlerFunc {
	h = withHeaders(h)
	h = middlewares.With4(s.Config, s.Queries, s.DB, s.Clock, h)
	// Register Sentry before Recover because it re-raises panics.
	if s.Config.SentryDSN != "" {
		h = Sentry(h)
	}
	// Register Recover after all middlewares but before logger
	// so that recover has access to the logger.
	if !s.Config.Debug {
		h = middlewares.Recover(h)
	}
	h = middlewares.WithLogger(s.Logger, h)
	return josh.Wrap(h)
}

// Add all app endpoints into the global HTTP mux.
func (s Server) RegisterEndpoints(mux *http.ServeMux) {
	router := s.getRouter()
	router.Register(mux)
}

func (s Server) getRouter() josh.Router {
	ValResp := schemas.ValidateResponse
	router := josh.Router{
		"/users/me": {
			// Get all information about the current user.
			GET: wrap(s, ValResp("me", getMe)),
			// Register a new user.
			POST: wrap(s, ValResp("me", addMe)),
			// Update user.
			PATCH: wrap(s, ValResp("me", updateMe)),
			// Soft-delete user.
			DELETE: wrap(s, deleteMe),
		},
		// Health checks.
		"/healthz/live":  {GET: Live},
		"/healthz/ready": {GET: Ready},
		"/healthz/start": {GET: Start},
	}
	if s.Config.Debug {
		router["/dev/token"] = josh.Endpoint{
			GET: josh.Wrap(middlewares.With2(s.Config, s.Logger, generateToken)),
		}
	}
	return router
}

// Equivalent to [ServerErrorC] but accepts request
// and doesn't log errors caused by client disconnecting.
func ServerErrorR(r josh.Req, msg string, err error) josh.Resp {
	ctx := r.Context()
	if errors.Is(err, context.Canceled) {
		select {
		case <-ctx.Done():
			return josh.NoResponse()
		default:
		}
	}
	return ServerErrorC(ctx, msg, err)
}

// Log the error and return InternalServerError response.
//
// The msg is included in both the response and the logs. It should be user-friendly.
// The err is included only in the logs. It should be informative for engineers.
//
// Each error includes a random code that can be used to map user complaints
// to log records.
func ServerErrorC(ctx context.Context, msg string, err error) josh.Resp {
	errCode := strconv.FormatInt(rand.Int64(), 36)[:8]

	// Write the error into the logs.
	// The logger should be always available in the context.
	// But if, for some reason, it's not, we don't want to explode the application.
	// So, we ignore it. And what else would you do, log it?!
	logger, _ := josh.CGetSingleton[*slog.Logger](ctx)
	if logger != nil {
		logger.ErrorContext(ctx, msg, "error", err, "error_code", errCode)
	}

	return josh.InternalServerError(josh.Error{
		Detail: msg,
		Code:   errCode,
	})
}
