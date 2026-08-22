package lib

import (
	"errors"

	"github.com/stripe/stripe-go/v84"
)

type Shipping = stripe.CheckoutSessionCreateShippingOptionShippingRateDataParams

const EUR = 100

func calculateShipping(country string, items []Item) (Shipping, error) {
	switch country {
	case "NL", "BE", "LU":
		var qty int64 = 0
		for _, item := range items {
			qty = int64(item.Qty)
		}
		cost := (9 + 3*qty) * EUR
		shipping := stripe.CheckoutSessionCreateShippingOptionShippingRateDataParams{
			DisplayName: new("Standard Benelux shipping via PostNL"),
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
		return Shipping{}, errors.New("cannot ship to the selected country")
	}
}
