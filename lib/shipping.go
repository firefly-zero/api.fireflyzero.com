package lib

import (
	"errors"

	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type StripeShipping = stripe.CheckoutSessionCreateShippingOptionShippingRateDataParams

type Shipping struct {
	Name string `json:"name"`
	Cost int64  `json:"cost"`
}

const EUR = 100

// POST /shipping
//
// Get shipping info (including costs) for the given country and items.
//
// Semantically equivalent to QUERY verb (GET with body)
// which is not well-supported by browsers just yet.
func queryShipping(r josh.Req) josh.Resp {
	body, err := schemas.ReadBody[CheckoutReq](r, "shipping", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes
	shipping, err := calculateShipping(attrs.Country, attrs.Items)
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	return josh.Ok(josh.Data[Shipping]{
		ID:   attrs.Country,
		Type: "shipping",
		Attributes: Shipping{
			Name: *shipping.DisplayName,
			Cost: *shipping.FixedAmount.Amount,
		},
	})
}

func calculateShipping(country string, items []Item) (StripeShipping, error) {
	switch country {
	case "NL", "BE", "LU":
		var qty int64 = 0
		for _, item := range items {
			qty = int64(item.Qty)
		}
		cost := (9 + 3*qty) * EUR
		shipping := stripe.CheckoutSessionCreateShippingOptionShippingRateDataParams{
			DisplayName: new("Standard shipping to " + country + " via PostNL"),
			DeliveryEstimate: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataDeliveryEstimateParams{
				Minimum: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataDeliveryEstimateMinimumParams{
					Unit:  new("day"),
					Value: new(int64(4)),
				},
				Maximum: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataDeliveryEstimateMaximumParams{
					Unit:  new("day"),
					Value: new(int64(28)),
				},
			},
			FixedAmount: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataFixedAmountParams{
				Amount:   &cost,
				Currency: new(string(stripe.CurrencyEUR)),
			},
			TaxBehavior: new("inclusive"),
			TaxCode:     new("txcd_92010001"),
			Type:        new("fixed_amount"),
		}
		return shipping, nil
	default:
		return StripeShipping{}, errors.New("cannot ship to the selected country")
	}
}
