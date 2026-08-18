package lib

import (
	"context"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Order struct {
	Paid bool `json:"paid"`
}

type CheckoutReq struct {
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
	Items      []Item `json:"items"`
}

type Item struct {
	ID       string `json:"id"`
	Quantity uint8  `json:"quantity"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

func checkoutOrder(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, len(attrs.Items))
	for i, item := range attrs.Items {
		lineItem := stripe.CheckoutSessionCreateLineItemParams{
			Price:    &item.ID,
			Quantity: new(int64(item.Quantity)),
			AdjustableQuantity: &stripe.CheckoutSessionCreateLineItemAdjustableQuantityParams{
				Enabled: new(true),
			},
		}
		lineItems[i] = &lineItem
	}

	// From this point on, the request cannot be canceled.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()

	// shipping := stripe.CheckoutSessionCreateShippingOptionParams{
	// 	ShippingRateData: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataParams{
	// 		DeliveryEstimate: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataDeliveryEstimateParams{},
	// 		DisplayName:      new("Standard EU shipping"),
	// 		FixedAmount: &stripe.CheckoutSessionCreateShippingOptionShippingRateDataFixedAmountParams{
	// 			Amount:   new(int64),
	// 			Currency: new(string(stripe.CurrencyEUR)),
	// 		},
	// 		TaxBehavior: new("inclusive"),
	// 		TaxCode:     new("txcd_92010001"),
	// 		Type:        new("fixed_amount"),
	// 	},
	// }
	params := stripe.CheckoutSessionCreateParams{
		Mode:       new(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: &attrs.SuccessURL,
		CancelURL:  &attrs.CancelURL,
		Customer:   new(string(customerID)),
		LineItems:  lineItems,
		ShippingAddressCollection: &stripe.CheckoutSessionCreateShippingAddressCollectionParams{
			AllowedCountries: []*string{new("NL")},
		},
		// ShippingOptions:     []*stripe.CheckoutSessionCreateShippingOptionParams{&shipping},
		AllowPromotionCodes: new(false),
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: new(true),
		},
		Currency: new(string(stripe.CurrencyEUR)),
	}
	session, err := client.V1CheckoutSessions.Create(ctx, &params)
	if err != nil {
		return ServerErrorR(r, "failed to create checkout session", err)
	}

	return josh.Ok(josh.Data[CheckoutResp]{
		ID:   session.ID,
		Type: "checkout",
		Attributes: CheckoutResp{
			RedirectURL: session.URL,
		},
	})
}
