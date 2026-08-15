package schemas

import (
	"github.com/orsinium-labs/jsony"
	. "github.com/orsinium-labs/valdo/valdo" //nolint:revive,staticcheck
)

const (
	datePattern = `(19|20)[0-9][0-9]-[01][0-9]-[0-3][0-9]`
	timePattern = `[0-2][0-9]:[0-5][0-9]:[0-5][0-9]`
	tzPattern   = `([+-][0-2][0-9]:[0-5][0-9]|Z)`
	dtPattern   = datePattern + "T" + timePattern + tzPattern
)

func init() {
	// id := Meta{
	// 	Validator: S(MinLen(1), MaxLen(18), Pattern(`^[1-9][0-9]*$`)),
	// 	Example:   jsony.SafeString("13"),
	// }
	datetime := Meta{
		Validator: S(MinLen(18), MaxLen(35), Pattern("^"+dtPattern+"$")),
		Example:   jsony.SafeString("2024-09-29T12:46:34+01:00"),
	}

	email := Meta{
		Validator:   S(MinLen(5), MaxLen(128)),
		Example:     jsony.SafeString("mail@example.com"),
		Description: "Used to match users between supabase and backend.",
	}
	language2 := Meta{
		Validator:   S(MinLen(2), MaxLen(2), Pattern(`^[a-z]{2}$`)),
		Example:     jsony.SafeString("en"),
		Description: "ISO 639-1 language code.",
	}
	timezone := Meta{
		Validator: S(MinLen(1), MaxLen(64)),
		Example:   jsony.SafeString("Europe/Amsterdam"),
	}
	country := S(MinLen(2), MaxLen(2), Pattern(`^[A-Z][A-Z]$`))
	add("me", "", O(
		P("email", email),
		P("country", country),
		P("language", language2),
		P("timezone", timezone),
		P("created_at", datetime),
		P("updated_at", datetime),
	))
	add("me", "_add", O(
		P("language", language2),
		P("country", country),
		P("timezone", timezone),
	))
	add("me", "_patch", O(
		P("language", language2).Optional(),
		P("country", country).Optional(),
		P("timezone", timezone).Optional(),
	))

	add("checkout", "_req", O(
		P("success_url", S(MinLen(11), MaxLen(200))),
		P("cancel_url", S(MinLen(11), MaxLen(200))),
	))
	add("checkout", "_resp", O(
		P("redirect_url", S(MinLen(11), MaxLen(200))),
	))

	errSchema := O(
		P("detail", Meta{
			Description: "User-friendly error message text.",
			Example:     jsony.SafeString("failed to save image"),
			Validator:   S(MinLen(1)),
		}),
		P("code", Meta{
			Description: "Error code that can be used in crash reports to find relevant backend logs.",
			Example:     jsony.SafeString("X98bHj12"),
			Validator:   S(MinLen(8), MaxLen(8)),
		}).Optional(),
		P("source", O(
			P("pointer", Meta{
				Description: "Path to the request body value that caused the error.",
				Example:     jsony.SafeString("/data/attributes/username"),
				Validator: S(
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
