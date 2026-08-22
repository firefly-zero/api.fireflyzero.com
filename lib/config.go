package lib

import "github.com/orsinium-labs/configenv"

type Config struct {
	// Supabase JWT token.
	AuthSecret string

	// The time when Docker image with the service was built.
	BuildTime string

	// The deployment color: "green" or "blue".
	//
	// Used in observability to see which instance is now active.
	Color string

	// Port on which to listen for HTTP requests. Defaults to 3000.
	Port int

	// Debug mode. Must be used on dev env only.
	//
	// Makes logs pretty and security forgiving.
	Debug bool

	// Environment name. Included in Sentry events.
	Env string

	// Secret connection string for sending events into Sentry.
	SentryDSN string

	// Secret key for accessing Stripe (payment system).
	StripeKey string
}

// Populate config from the passed env var pairs.
func (c *Config) ParseEnv(pairs []string) error {
	vars := configenv.Vars{
		"DEBUG":       configenv.Required(configenv.Bool(&c.Debug)),
		"AUTH_SECRET": configenv.Required(configenv.String(&c.AuthSecret)),
		"BUILD_TIME":  configenv.Required(configenv.String(&c.BuildTime)),
		"COLOR":       configenv.Required(configenv.String(&c.Color)),
		"SENTRY_DSN":  configenv.String(&c.SentryDSN),
		"STRIPE_KEY":  configenv.String(&c.StripeKey),
		"ENV":         configenv.String(&c.Env),
		"PORT":        configenv.Required(configenv.Int(&c.Port)),
	}
	return vars.Parse(configenv.Config{
		Prefix:  "API_",
		Environ: pairs,
	})
}
