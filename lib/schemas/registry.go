package schemas

import (
	"github.com/orsinium-labs/jsony"
	. "github.com/orsinium-labs/valdo/valdo" //nolint:revive,stylecheck
)

var registry = map[string]ObjectType{}

func Get(name string) ObjectType {
	v, found := registry[name]
	if !found {
		panic(name + " schema not found")
	}
	return v
}

func add(name, suffix string, v Validator) {
	data := wrapData(name, suffix, v)
	if suffix == "_add" || (name == "me" && suffix == "_patch") {
		data = O(
			P("type", Const(name)),
			P("attributes", v),
		)
	}
	registry[name+suffix] = O(P("data", data))
	if suffix == "" {
		plural := name + suffix + "s"
		registry[plural] = O(P("data", A(data)))
	}
}

func wrapData(name, suffix string, v Validator) Validator {
	id := Meta{
		Validator: String(MinLen(1), MaxLen(18), Pattern(`^[1-9][0-9]*$`)),
		Example:   jsony.SafeString("13"),
	}
	data := O(
		P("id", id),
		P("type", Const(name)),
		P("attributes", v),
	)
	if suffix == "_add" || (name == "me" && suffix == "_patch") {
		data = O(
			P("type", Const(name)),
			P("attributes", v),
		)
	}
	return data
}
