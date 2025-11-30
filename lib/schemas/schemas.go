package schemas

import (
	"github.com/orsinium-labs/jsony"
	. "github.com/orsinium-labs/valdo/valdo" //nolint:revive,stylecheck
)

const (
	datePattern   = `(19|20)[0-9][0-9]-[01][0-9]-[0-3][0-9]`
	timePattern   = `[0-2][0-9]:[0-5][0-9]:[0-5][0-9]`
	tzPattern     = `([+-][0-2][0-9]:[0-5][0-9]|Z)`
	dtPattern     = datePattern + "T" + timePattern + tzPattern
	base64Pattern = `[a-zA-Z0-9\+\/\=]+`
)

func init() {
	datetime := Meta{
		Validator: String(MinLen(18), MaxLen(35), Pattern("^"+dtPattern+"$")),
		Example:   jsony.SafeString("2024-09-29T12:46:34+01:00"),
	}

	email := Meta{
		Validator:   String(MinLen(5), MaxLen(128)),
		Example:     jsony.SafeString("mail@example.com"),
		Description: "Used to match users between supabase and backend.",
	}
	userName := Meta{
		Validator: String(
			MinLen(2), MaxLen(32),
			// Alphanumeric and may contain "_.-" but not at the beginning or end.
			Pattern(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*[a-zA-Z0-9]$`),
		),
		Example:     jsony.SafeString("snz"),
		Description: "The unique display name.",
	}
	language2 := Meta{
		Validator:   String(MinLen(2), MaxLen(2), Pattern(`^[a-z]{2}$`)),
		Example:     jsony.SafeString("en"),
		Description: "ISO 639-1 language code.",
	}
	timezone := Meta{
		Validator: String(MinLen(1), MaxLen(64)),
		Example:   jsony.SafeString("Europe/Amsterdam"),
	}
	country := String(MinLen(2), MaxLen(2), Pattern(`^[A-Z][A-Z]$`))
	add("me", "", O(
		P("email", email),
		P("name", userName),
		P("country", country),
		P("language", language2),
		P("timezone", timezone),
		P("created_at", datetime),
		P("updated_at", datetime),
	))
	add("me", "_add", O(
		P("name", userName),
		P("language", language2),
		P("country", country),
		P("timezone", timezone),
	))
	add("me", "_patch", O(
		P("language", language2).Optional(),
		P("country", country).Optional(),
		P("timezone", timezone).Optional(),
	))

	errSchema := O(
		P("detail", Meta{
			Description: "User-friendly error message text.",
			Example:     jsony.SafeString("failed to save image"),
			Validator:   String(MinLen(1)),
		}),
		P("code", Meta{
			Description: "Error code that can be used in crash reports to find relevant backend logs.",
			Example:     jsony.SafeString("X98bHj12"),
			Validator:   String(MinLen(8), MaxLen(8)),
		}).Optional(),
		P("source", O(
			P("pointer", Meta{
				Description: "Path to the request body value that caused the error.",
				Example:     jsony.SafeString("/data/attributes/username"),
				Validator: String(
					MinLen(2), MaxLen(64),
					Pattern(`\/[a-z_\/]+`),
				),
			}),
		)).Optional(),
	)
	registry["error"] = errSchema
	registry["failure"] = O(
		P("errors", A(errSchema, MinItems(1))),
	)
}
