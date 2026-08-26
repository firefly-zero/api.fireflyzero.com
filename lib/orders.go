package lib

import (
	"strings"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Order struct {
	Paid      bool        `json:"paid"`
	Amount    int64       `json:"amount"`
	Currency  string      `json:"currency"`
	CreatedAt string      `json:"created_at"`
	Items     []OrderItem `json:"items"`
}

type OrderItem struct {
	Name string `json:"name"`
	Qty  int64  `json:"qty"`
}

type CheckoutReq struct {
	Country    string `json:"country"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
	Promotion  string `json:"promotion"`
	Items      []Item `json:"items"`
}

type Item struct {
	ID  string `json:"id"`
	Qty uint8  `json:"qty"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

func checkoutOrder(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, len(attrs.Items))
	for i, item := range attrs.Items {
		lineItem := stripe.CheckoutSessionCreateLineItemParams{
			Price:    &item.ID,
			Quantity: new(int64(item.Qty)),
		}
		lineItems[i] = &lineItem
	}

	shipping, err := calculateShipping(attrs.Country, attrs.Items)
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}

	// If a promo code is provided, check it and activate the discount.
	var discounts []*stripe.CheckoutSessionCreateDiscountParams
	if attrs.Promotion != "" {
		code := strings.TrimSpace(attrs.Promotion)
		code = strings.ReplaceAll(code, " ", "_")
		codes, err := loadStripeList(client.V1PromotionCodes.List(r.Context(), &stripe.PromotionCodeListParams{
			Active: new(true),
			Code:   &code,
		}))
		if err != nil {
			return ServerErrorR(r, "failed to check promotion code", err)
		}
		if len(codes) == 0 {
			return josh.BadRequest(josh.Error{
				Detail: "unknown promotion code",
			})
		}
		codeInfo := codes[0]
		if codeInfo.Customer != nil && codeInfo.Customer.ID != string(customerID) {
			return josh.BadRequest(josh.Error{
				Detail: "this promotion code is for someone else",
			})
		}
		discounts = []*stripe.CheckoutSessionCreateDiscountParams{{
			PromotionCode: &codeInfo.ID,
		}}
	}

	params := stripe.CheckoutSessionCreateParams{
		Mode:       new(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: &attrs.SuccessURL,
		CancelURL:  &attrs.CancelURL,
		Customer:   new(string(customerID)),
		LineItems:  lineItems,
		// Since we already calculated shipping costs,
		// don't let the user change the shipping address.
		ShippingAddressCollection: &stripe.CheckoutSessionCreateShippingAddressCollectionParams{
			AllowedCountries: []*string{&attrs.Country},
		},
		ShippingOptions: []*stripe.CheckoutSessionCreateShippingOptionParams{{
			ShippingRateData: &shipping,
		}},
		AutomaticTax: &stripe.CheckoutSessionCreateAutomaticTaxParams{
			Enabled: new(true),
		},
		Discounts: discounts,
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Address:  new("auto"),
			Name:     new("auto"),
			Shipping: new("auto"),
		},
		Currency: new(string(stripe.CurrencyEUR)),
	}
	session, err := client.V1CheckoutSessions.Create(r.Context(), &params)
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

func getOrder(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	orderID := r.PathValue("order")
	session, err := client.V1CheckoutSessions.Retrieve(r.Context(), orderID, &stripe.CheckoutSessionRetrieveParams{
		Expand: []*string{new("line_items")},
	})
	if err != nil {
		return ServerErrorR(r, "failed to get order info", err)
	}
	if session.Customer.ID != string(customerID) {
		return josh.Forbidden(josh.Error{
			Detail: "you don't have access to the given order",
		})
	}
	createdAt := time.Unix(session.Created, 0)

	items := make([]OrderItem, 0, len(session.LineItems.Data))
	for _, item := range session.LineItems.Data {
		items = append(items, OrderItem{
			Name: item.Description,
			Qty:  item.Quantity,
		})
	}

	return josh.Ok(josh.Data[Order]{
		ID:   orderID[8:24],
		Type: "order",
		Attributes: Order{
			Paid:      session.PaymentStatus == "paid",
			Amount:    session.AmountTotal,
			Currency:  string(session.Currency),
			CreatedAt: createdAt.Format(time.RFC3339),
			Items:     items,
		},
	})
}
