package schemas

import (
	"github.com/orsinium-labs/jsony"
	. "github.com/orsinium-labs/valdo/valdo" //nolint:revive,staticcheck
)

func init() {
	country := S(MinLen(2), MaxLen(2), Pattern(`^[A-Z][A-Z]$`))
	item := O(
		P("id", S(MinLen(7))),
		P("qty", Int(Min(1))),
	)
	add("checkout", "_add", O(
		P("country", country),
		P("success_url", S(MinLen(11), MaxLen(200))),
		P("cancel_url", S(MinLen(11), MaxLen(200))),
		P("promotion", Nullable(S(MinLen(1), MaxLen(64)))),
		P("items", Array(item)),
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
