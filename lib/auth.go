package lib

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/orsinium-labs/josh"
)

type jwtClaims struct {
	jwt.RegisteredClaims
	Email       string `json:"email"`
	AppMetadata struct {
		CustomerID CustomerID `json:"customer_id"`
	} `json:"app_metadata"`
}

type JWT struct {
	Email      string
	SupabaseID uuid.UUID
	CustomerID CustomerID
}

const (
	// For how long should live the token issued on dev.
	jwtTTL    = 8 * time.Hour
	jwkSetURL = "https://kxllbgiurjbkwtjarkec.supabase.co/auth/v1/.well-known/jwks.json"
)

func newKeyFunc(ctx context.Context, config Config) jwt.Keyfunc {
	// TODO(@orsinium): pass http.Client from main.go
	jwkSet := josh.Must(keyfunc.NewDefaultCtx(ctx, []string{jwkSetURL}))
	return func(token *jwt.Token) (any, error) {
		alg, _ := token.Header["alg"].(string)
		if alg == "HS256" {
			return []byte(config.AuthSecret), nil
		}
		return jwkSet.Keyfunc(token)
	}
}

// Validate JWT token and extract the user email.
func authValidator(keyFunc jwt.Keyfunc) func(string) (JWT, error) {
	return func(rawToken string) (JWT, error) {
		token, err := jwt.ParseWithClaims(
			rawToken,
			&jwtClaims{},
			keyFunc,
			jwt.WithValidMethods([]string{"HS256", "ES256"}),
			jwt.WithIssuedAt(),
			jwt.WithLeeway(5*time.Second),
		)
		if err != nil {
			return JWT{}, fmt.Errorf("cannot parse JWT token: %v", err)
		}
		claims, ok := token.Claims.(*jwtClaims)
		if !ok {
			return JWT{}, errors.New("invalid JWT claims format")
		}
		expiration, _ := claims.GetExpirationTime()
		if expiration == nil {
			return JWT{}, errors.New("the JWT token doesn't have expiration time set")
		}
		if claims.Email == "" {
			return JWT{}, errors.New("the JWT token doesn't contain user email")
		}
		supabaseID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return JWT{}, errors.New("invalid Supabase user ID")
		}
		return JWT{
			Email:      claims.Email,
			SupabaseID: supabaseID,
			CustomerID: claims.AppMetadata.CustomerID,
		}, nil
	}
}

type TokenResp struct {
	Token string `json:"token"`
}

// Generate a new valid JWT token.
//
// Available only on dev. Used to generate auth header for testing purposes.
func generateToken(r josh.Req) josh.Resp {
	config := josh.Must(josh.GetSingleton[Config](r))
	if !config.Debug {
		panic("GET token endpoint must be available only on dev")
	}
	email := r.URL.Query().Get("email")
	if email == "" {
		return josh.BadRequest(josh.Error{Detail: "email is required"})
	}
	if !strings.Contains(email, "@") {
		return josh.BadRequest(josh.Error{Detail: "invalid email address"})
	}
	token := newJWT(email)
	rawToken, err := token.SignedString([]byte(config.AuthSecret))
	if err != nil {
		return ServerErrorR(r, "token could not be signed", err)
	}
	return josh.Ok(josh.Data[TokenResp]{
		ID:         email,
		Type:       "token",
		Attributes: TokenResp{rawToken},
	})
}

func newJWT(email string) *jwt.Token {
	now := time.Now()
	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"email": email,
			"sub":   "5e8b5f5a-7404-4346-8a0c-7b48e34c94d5",
			// Issued at
			"iat": float64(now.Unix()),
			// Expires at
			"exp": float64(now.Add(jwtTTL).Unix()),
			// Not valid before
			"nbf": float64(now.Add(-time.Second).Unix()),
			// Issued by
			"iss": "api",
		},
	)
	return token
}
