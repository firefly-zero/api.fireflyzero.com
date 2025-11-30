package schemas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"github.com/orsinium-labs/josh"
	"github.com/orsinium-labs/valdo/valdo"
)

func ReadBody[T any](r josh.Req, name, suffix string) (josh.Data[T], error) {
	schema := Get(name + suffix)
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		return josh.Data[T]{}, fmt.Errorf("cannot read request: %v", err)
	}
	body, err := josh.Read[T](name, bytes.NewReader(rawBody))
	if err != nil {
		return body, fmt.Errorf("cannot parse request: %v", err)
	}
	err = valdo.Validate(schema, rawBody)
	if err != nil {
		return body, fmt.Errorf("invalid request: %v", err)
	}
	return body, nil
}

// Read ID from URL and compare it to the ID in the request body.
func ParseID[T any](name string, r josh.Req, body josh.Data[T]) (int32, josh.Resp) {
	idURL, errResp := josh.GetID[int32](r, name)
	if errResp != nil {
		return 0, josh.BadRequest(*errResp)
	}
	if body.ID == "" {
		return 0, josh.BadRequest(josh.Error{Detail: "the id is required"})
	}
	id64, err := strconv.ParseInt(body.ID, 10, 32)
	if err != nil {
		return 0, josh.BadRequest(josh.Error{Detail: "the id must be an integer number"})
	}
	id := int32(id64)
	if id <= 0 {
		return 0, josh.BadRequest(josh.Error{Detail: "the id must be positive"})
	}
	if id != idURL {
		return 0, josh.BadRequest(josh.Error{Detail: "the IDs in the URL and in the body don't match"})
	}
	return id, josh.NoResponse()
}

// Middleware validating the endpoint response against the given schema.
func ValidateResponse(name string, h josh.Handler) josh.Handler {
	schema := Get(name)
	return func(r josh.Req) josh.Resp {
		logger := josh.Must(josh.GetSingleton[*slog.Logger](r))
		resp := h(r)
		if resp.Data != nil {
			raw, _ := json.Marshal(resp)
			err := valdo.Validate(schema, raw)
			if err != nil {
				logger.WarnContext(
					r.Context(), "response payload doesn't match the schema",
					"schema", name,
					"error", err,
				)
			}
		}
		return resp
	}
}
