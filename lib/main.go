package lib

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/rs/cors"
	"github.com/stripe/stripe-go/v84"
)

func Run() error {
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
	logger := slog.New(logHandler)

	httpServer := setup(
		config,
		logger,
	)
	return httpServer.ListenAndServe()
}

func setup(
	config Config,
	logger *slog.Logger,
) *http.Server {
	if config.Debug {
		logger.Warn("running in debug mode!")
	}
	server := Server{
		Logger:  logger,
		Config:  config,
		KeyFunc: newKeyFunc(context.Background(), config),
		Clock:   time.Now,
		Stripe:  stripe.NewClient(config.StripeKey),
	}
	mux := http.NewServeMux()
	server.RegisterEndpoints(mux)

	// setup CORS
	corsConfig := cors.Options{
		AllowedMethods:      []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:      []string{"*"},
		AllowCredentials:    true,
		MaxAge:              3600, // 1 hour
		AllowPrivateNetwork: config.Debug,
	}
	if !config.Debug {
		corsConfig.AllowedOrigins = []string{"https://*.fireflyzero.com"}
	}
	cors := cors.New(corsConfig)
	handler := cors.Handler(mux)

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
