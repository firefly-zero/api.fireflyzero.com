package lib_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib"
	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/lmittmann/tint"
	"github.com/matryer/is"
	"github.com/orsinium-labs/josh"
)

type TestUser struct {
	client TestClient
	email  string
	token  string
}

func NewTestUser(client TestClient, email string) TestUser {
	return TestUser{client: client, email: email}
}

type TestClock struct {
	now *time.Time
	loc *time.Location
}

func (c *TestClock) Now() time.Time {
	if c.now != nil {
		return *c.now
	}
	return time.Now()
}

type TestClient struct {
	server   *httptest.Server
	is       *is.I
	t        *testing.T
	queries  *db.Queries
	Validate bool
	clock    *TestClock
}

func NewTestClient(t *testing.T) TestClient {
	t.Helper()
	is := is.New(t)
	conn0, err := pgx.Connect(t.Context(), os.Getenv("API_POSTGRES_URL"))
	is.NoErr(err)
	tx, err := conn0.BeginTx(t.Context(), pgx.TxOptions{})
	conn := NewBlockingDB(tx)
	is.NoErr(err)

	logger := slog.New(newTestLogHandler(t))

	clock := &TestClock{}
	queries := db.New(conn)
	is.NoErr(err)
	server := lib.Server{
		Logger: logger,
		Config: lib.Config{
			AuthSecret: "TEST_SECRET",
			BuildTime:  "2023-12-31",
			Debug:      true,
			Color:      "blue",
		},
		Queries: queries,
		DB:      tx,
		Clock:   clock.Now,
	}
	mux := http.NewServeMux()
	server.RegisterEndpoints(mux)
	client := TestClient{
		server:   httptest.NewServer(mux),
		is:       is,
		t:        t,
		queries:  server.Queries,
		Validate: true,
		clock:    clock,
	}
	t.Cleanup(func() {
		conn.Map(func(db.DBTX) {
			ctx := context.Background() //nolint:usetesting
			is.NoErr(tx.Rollback(ctx))
			is.NoErr(conn0.Close(ctx))
		})
		client.server.Close()
	})
	return client
}

// Setup a colorful log handler.
func newTestLogHandler(t *testing.T) slog.Handler {
	t.Helper()
	handler := tint.NewTextHandler(os.Stdout, &tint.Options{Level: slog.LevelDebug, ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		err, isError := a.Value.Any().(error)
		if isError {
			aErr := tint.Err(err)
			aErr.Key = a.Key
			return aErr
		}
		return a
	}})
	return LogTestHandler{handler, t}
}

type LogTestHandler struct {
	h slog.Handler
	t *testing.T
}

// Enabled implements slog.Handler.
func (h LogTestHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.h.Enabled(ctx, lvl)
}

// WithAttrs implements slog.Handler.
func (h LogTestHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := h.h.WithAttrs(attrs)
	return LogTestHandler{newHandler, h.t}
}

// WithGroup implements slog.Handler.
func (h LogTestHandler) WithGroup(name string) slog.Handler {
	newHandler := h.h.WithGroup(name)
	return LogTestHandler{newHandler, h.t}
}

func (h LogTestHandler) Handle(ctx context.Context, rec slog.Record) error {
	err := h.h.Handle(ctx, rec)
	if rec.Level == slog.LevelError {
		h.t.Errorf("unexpected error logged: %v", rec.Message)
	}
	if rec.Level == slog.LevelWarn {
		h.t.Errorf("unexpected warning logged: %v", rec.Message)
	}
	return err
}

func (c *TestClient) SetDate(d string) {
	now := josh.Must(time.ParseInLocation(time.DateOnly, d, c.clock.loc))
	c.clock.now = &now
}

func (c *TestClient) SetTimezone(tz string) {
	loc, err := time.LoadLocation(tz)
	c.is.NoErr(err)
	c.clock.loc = loc
}

func (c *TestClient) SetTime(d string) {
	now := josh.Must(time.Parse(time.TimeOnly, d))
	if c.clock.now == nil {
		now := time.Now()
		c.clock.now = &now
	}
	old := *c.clock.now
	now = time.Date(
		old.Year(), old.Month(), old.Day(),
		now.Hour(), now.Minute(), now.Second(),
		0, c.clock.loc,
	)
	c.clock.now = &now
}

func (c *TestClient) GetAnon(path string) *http.Response {
	return c.DoAnon("GET", path, "")
}

// Send anonymous request.
func (c *TestClient) DoAnon(method, path, body string) *http.Response {
	client := c.server.Client()
	req, err := http.NewRequest(method, c.server.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("X-Api-Version", "2025-06-09")
	req.Header.Set("X-App-Platform", "linux")
	req.Header.Set("X-App-Version", "1.2.3")
	c.is.NoErr(err)
	resp, err := client.Do(req)
	c.is.NoErr(err)
	req.Body = io.NopCloser(bytes.NewBufferString(body))
	c.t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (u *TestUser) GetUser(path string) *http.Response {
	return u.DoUser("GET", path, "")
}

// Send request using a test user.
func (u *TestUser) DoUser(method, path, body string) *http.Response {
	return u.DoUserReader(method, path, bytes.NewBufferString(body))
}

func (u *TestUser) DoUserReader(method, path string, body io.Reader) *http.Response {
	c := u.client
	u.fetchToken()
	client := c.server.Client()
	req, err := http.NewRequest(method, c.server.URL+path, body)
	c.is.NoErr(err)
	req.Header.Set("Authorization", "Bearer "+u.token)
	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("X-Api-Version", "2025-06-09")
	resp, err := client.Do(req)
	c.is.NoErr(err)
	return resp
}

func (u *TestUser) fetchToken() {
	c := u.client
	c.t.Helper()
	if u.token != "" {
		return
	}
	if u.email == "" {
		c.t.Fatal("email is not provided")
	}
	resp := c.DoAnon("GET", "/dev/token?email="+u.email, "")
	c.is.Equal(resp.StatusCode, 200)
	body := map[string]map[string]any{}
	err := json.NewDecoder(resp.Body).Decode(&body)
	c.is.NoErr(err)
	attrs := body["data"]["attributes"].(map[string]any)
	u.token = attrs["token"].(string)
}

// Register the user associated with this test client.
func (u *TestUser) Register() {
	c := u.client
	reqBody := `{"data": {
		"type": "me",
		"attributes": {
			"language": "en",
			"country": "NL",
			"timezone": "Europe/Amsterdam"
		}
	}}`
	r := u.DoUser("POST", "/users/me", reqBody)
	EqBody(c.is, r, `{"data": ANY}`)
	c.is.Equal(r.StatusCode, 201)
}

func (u *TestUser) GetMe() map[string]any {
	c := u.client
	r := u.DoUser("GET", "/users/me", ``)
	body := EqBody(u.client.is, r, `{"data": ANY}`)
	c.is.Equal(r.StatusCode, 200)
	return GetResource(body)
}

// Assert that the response body is equal to the expected JSON.
func EqBody(is *is.I, r *http.Response, exp string) any {
	is.Helper()
	var got any
	err := json.NewDecoder(r.Body).Decode(&got)
	is.NoErr(err)
	EqObject(is, got, exp)
	return got
}

// Assert that the given object (of any type) is equal to the expected JSON.
func EqObject(is *is.I, got any, exp string) {
	is.Helper()
	var want any
	exp = strings.ReplaceAll(exp, "ANY", `"ANY"`)
	err := json.Unmarshal([]byte(exp), &want)
	is.NoErr(err)

	diff := cmp.Diff(
		want, got,
		// If the expected value is -1, allow any actual value.
		// It is used to discard IDs and dates from the input.
		cmp.FilterValues(func(a, b any) bool {
			return a == "ANY" || b == "ANY"
		}, cmp.Ignore()),
	)
	if diff != "" {
		fmt.Println(diff)
		is.Fail()
	}
}

// Ectract a Resource object from JSON:API response body.
func GetResource(body any) map[string]any {
	switch b := body.(type) {
	case map[string]any:
		if b["errors"] != nil {
			panic(fmt.Sprintf("error response: %v", b["errors"]))
		}
		if b["data"] == nil {
			panic(fmt.Sprintf("response has nil data: %v", b))
		}
		return b["data"].(map[string]any)
	case josh.Resp:
		if b.Errors != nil {
			panic(fmt.Sprintf("error response: %v", b.Errors))
		}
		if b.Data == nil {
			panic(fmt.Sprintf("response has nil data: %v", b))
		}
		return b.Data.(map[string]any)
	default:
		panic(fmt.Sprintf("invalid body type: %T", body))
	}
}

// Ectract list of Resource objects from JSON:API response body.
func GetResources(body any) []map[string]any {
	b := body.(map[string]any)
	if b["errors"] != nil {
		panic(fmt.Sprintf("error response: %v", b["errors"]))
	}
	if b["data"] == nil {
		panic(fmt.Sprintf("response has nil data: %v", b))
	}
	resources := b["data"].([]any)
	result := make([]map[string]any, 0, len(resources))
	for _, res := range resources {
		result = append(result, res.(map[string]any))
	}
	return result
}

func GetIDs(resources []map[string]any) []int32 {
	result := make([]int32, 0, len(resources))
	for _, resource := range resources {
		result = append(result, GetID(resource))
	}
	return result
}

// Get object ID from a raw resource object.
func GetID(resource map[string]any) int32 {
	strID := resource["id"].(string)
	return int32(josh.Must(strconv.ParseInt(strID, 10, 32)))
}

func GetAttr(resource map[string]any, path string) string {
	attrs := resource["attributes"].(map[string]any)
	return fmt.Sprintf("%v", attrs[path])
}
