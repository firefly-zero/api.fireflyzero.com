package lib

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

var rexSQLName = regexp.MustCompile(`\-\-\s*name\:\s+[A-Za-z0-9]+`)

func Run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config := Config{}
	err := config.ParseEnv(os.Environ())
	if err != nil {
		return fmt.Errorf("read config: %v", err)
	}

	if config.SentryDSN != "" {
		err = sentry.Init(sentry.ClientOptions{
			Dsn:              config.SentryDSN,
			Debug:            config.Debug,
			AttachStacktrace: true,
			Release:          config.BuildTime,
			Environment:      config.Env,
		})
		if err != nil {
			return fmt.Errorf("setup Sentry: %v", err)
		}
	}
	logHandler := NewLogHandler(config)
	if config.Env != "" {
		otel, err := NewOpenTelemetry(config)
		if err != nil {
			return fmt.Errorf("setup OpenTelemetry: %v", err)
		}
		defer otel.Close()
		err = runtime.Start(
			runtime.WithMinimumReadMemStatsInterval(time.Second),
		)
		if err != nil {
			return fmt.Errorf("start OpenTelemetry runtime instrumentation: %v", err)
		}
		logHandler = otel.WrapLogHandler(logHandler)
	}
	logger := slog.New(logHandler)

	// Setup DB connection.
	pgxConf, err := pgxpool.ParseConfig(config.PostgresURL)
	if err != nil {
		return fmt.Errorf("parse PostgreSQL URL: %v", err)
	}
	pgxConf.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTrimSQLInSpanName(), // required for the SpanNameFunc to be called
		otelpgx.WithSpanNameFunc(getSQLSpanName),
	)
	dbpool, err := pgxpool.NewWithConfig(ctx, pgxConf)
	if err != nil {
		return fmt.Errorf("create connection pool: %v", err)
	}
	defer dbpool.Close()

	queries := db.New(dbpool)

	httpServer := setup(
		config,
		logger,
		queries,
		dbpool,
	)
	cancel()
	return httpServer.ListenAndServe()
}

func getSQLSpanName(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	name := rexSQLName.FindString(stmt)
	if name != "" {
		return name
	}
	return stmt
}

func setup(
	config Config,
	logger *slog.Logger,
	queries *db.Queries,
	dbpool Database,
) *http.Server {
	if config.Debug {
		logger.Warn("running in debug mode!")
	}
	server := Server{
		Logger:  logger,
		Config:  config,
		Queries: queries,
		DB:      dbpool,
		Clock:   time.Now,
	}
	mux := http.NewServeMux()
	server.RegisterEndpoints(mux)

	// setup CORS
	corsConfig := cors.Options{
		AllowedMethods:      []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:      []string{"*"},
		AllowCredentials:    false,
		MaxAge:              3600, // 1 hour
		AllowPrivateNetwork: config.Debug,
	}
	// TODO(@orsinium): enable AllowOrigins, set it to our prod app URL.
	// if !config.Debug {
	// 	corsConfig.AllowedOrigins = []string{"https://*.glind.app"}
	// }
	cors := cors.New(corsConfig)
	handler := otelhttp.NewHandler(mux, "rest-api")
	handler = cors.Handler(handler)

	httpServer := &http.Server{
		Handler:           handler,
		Addr:              fmt.Sprintf(":%d", config.Port),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    10 * 1024,
	}
	server.Logger.Info(
		"listening",
		"addr", httpServer.Addr,
		"build_time", config.BuildTime,
	)
	return httpServer
}
