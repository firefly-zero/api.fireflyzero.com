package lib

import (
	"encoding/json"
	"net/http"

	"github.com/orsinium-labs/josh"
)

func getExchangeRates(r josh.Req) josh.Resp {
	client := http.Client{}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://open.er-api.com/v6/latest/EUR", nil)
	if err != nil {
		return ServerErrorR(r, "failed to request exchange rates", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ServerErrorR(r, "failed to fetch exchange rates", err)
	}
	var body map[string]any
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return ServerErrorR(r, "failed to parse exchange rates", err)
	}
	return josh.Ok(josh.Data[any]{
		ID:         "eur",
		Type:       "rates",
		Attributes: body["rates"],
	})
}
