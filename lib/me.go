package lib

import (
	"context"
	"strconv"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/orsinium-labs/josh"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/text/language"
)

const defaultTimezone = "Europe/Amsterdam"

type Me struct {
	Email     string   `json:"email"`
	AuthorIDs []string `json:"author_ids"`
	Language  string   `json:"language"`
	Country   string   `json:"country"`
	Timezone  string   `json:"timezone"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func withMe(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		jwt := josh.Must(josh.GetSingleton[JWT](r))
		queries := josh.Must(josh.GetSingleton[*db.Queries](r))

		myID := jwt.UserID

		isRegistration := r.Pattern == "POST /users/me"
		if isRegistration {
			if myID != 0 {
				return josh.BadRequest(josh.Error{
					Detail: "already registered",
					Code:   "registered",
				})
			}
			return h(r)
		}

		if myID == 0 {
			user, err := queries.GetUserByEmail(r.Context(), jwt.Email)
			if err == pgx.ErrNoRows {
				return josh.Unauthorized(josh.Error{
					Detail: "user registration is not complete",
					Code:   "unregistered",
				})
			}
			if err != nil {
				return ServerErrorR(r, "failed to get user", err)
			}
			if user.DeletedAt.Valid {
				return josh.Unauthorized(josh.Error{
					Detail: "user is deleted",
					Code:   "deleted",
				})
			}
			myID = user.ID
		}

		r = josh.Must(josh.WithSingleton(r, myID))
		strID := strconv.FormatInt(int64(myID), 10)

		// https://docs.sentry.io/platforms/go/enriching-events/identify-user/
		hub := sentry.GetHubFromContext(r.Context()) //nolint:contextcheck
		if hub != nil {
			scope := hub.Scope()
			if scope != nil {
				scope.SetUser(sentry.User{
					ID:    strID,
					Email: jwt.Email,
				})
			}
		}

		span := trace.SpanFromContext(r.Context()) //nolint:contextcheck
		span.SetAttributes(
			semconv.UserEmail(jwt.Email),
			semconv.UserID(strID),
		)

		return h(r)
	}
}

func getMe(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	me, err := queries.GetUserByID(r.Context(), myID)
	if err == pgx.ErrNoRows {
		return josh.Unauthorized(josh.Error{
			Detail: "user is not created yet",
			Code:   "unregistered",
		})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get user info", err)
	}
	return josh.Ok(formatMe(me))
}

func addMe(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	jwt := josh.Must(josh.GetSingleton[JWT](r))

	body, err := schemas.ReadBody[Me](r, "me", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	// Additional validation.
	_, err = queries.GetUserByEmail(r.Context(), jwt.Email)
	if err != pgx.ErrNoRows {
		if err != nil {
			return ServerErrorR(r, "failed to check email availability", err)
		}
		return josh.BadRequest(josh.Error{
			Detail: "user with the given email is already registered",
			Source: josh.SourcePointer("/data/attributes/email"),
		})
	}

	// Set defaults.
	if attrs.Timezone == "" {
		attrs.Timezone = defaultTimezone
	}
	_, err = time.LoadLocation(attrs.Timezone)
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: "unsupported timezone"})
	}

	// Save the user in the DB.
	params := db.CreateUserParams{
		Email:    jwt.Email,
		Language: normalizeLanguage(attrs.Language),
		Country:  attrs.Country,
		Timezone: attrs.Timezone,
	}
	me, err := queries.CreateUser(r.Context(), params)
	if err != nil {
		return ServerErrorR(r, "failed to create user", err)
	}

	return josh.Created(formatMe(me))
}

func updateMe(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	me, err := queries.GetUserByID(r.Context(), myID)
	if err == pgx.ErrNoRows {
		return josh.Unauthorized(josh.Error{
			Detail: "user is not created yet",
			Code:   "unregistered",
		})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get user info", err)
	}

	body, err := schemas.ReadBody[Me](r, "me", "_patch")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	// Construct update SQL query.
	params := db.UpdateUserParams{
		ID: me.ID,
	}
	if attrs.Language != "" {
		lang := normalizeLanguage(attrs.Language)
		if me.Language != lang {
			params.Language = &lang
		}
	}
	if attrs.Timezone != "" && attrs.Timezone != me.Timezone {
		_, err = time.LoadLocation(attrs.Timezone)
		if err != nil {
			return josh.BadRequest(josh.Error{
				Detail: "unsupported timezone",
				Source: josh.SourcePointer("/data/attributes/timezone"),
			})
		}
		params.Timezone = &attrs.Timezone
	}
	if attrs.Country != "" && attrs.Country != me.Country {
		params.Country = &attrs.Country
	}

	newMe, err := queries.UpdateUser(r.Context(), params)
	if err != nil {
		return ServerErrorR(r, "failed to update user", err)
	}

	return josh.Ok(formatMe(newMe))
}

func deleteMe(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	clock := josh.Must(josh.GetSingleton[Clock](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()

	now := clock()
	err := queries.SoftDeleteUser(ctx, db.SoftDeleteUserParams{
		Now: pgtype.Timestamptz{Time: now, Valid: true},
		ID:  myID,
	})
	if err != nil {
		ServerErrorR(r, "failed to delete user", err)
	}

	return josh.NoContent()
}

func normalizeLanguage(lang string) string {
	parsed, err := language.Parse(lang)
	if err != nil {
		return "en"
	}
	base, _ := parsed.Base()
	baseS := base.String()
	if len(baseS) != 2 {
		return "en"
	}
	return baseS
}

func formatMe(me db.User) josh.Data[Me] {
	tz := me.Timezone
	_, err := time.LoadLocation(tz)
	if err != nil {
		tz = defaultTimezone
	}
	authorIDs := me.AuthorIds
	if authorIDs == nil {
		authorIDs = []string{}
	}
	return josh.Data[Me]{
		ID:   strconv.FormatInt(int64(me.ID), 10),
		Type: "me",
		Attributes: Me{
			Email:     me.Email,
			AuthorIDs: authorIDs,
			Language:  me.Language,
			Country:   me.Country,
			Timezone:  tz,
			CreatedAt: formatDateTime(me.CreatedAt),
			UpdatedAt: formatDateTime(me.UpdatedAt),
		},
	}
}
