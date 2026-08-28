package lib

import (
	"fmt"
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
	// The country code where the order will be shipped.
	//
	// That's the only part of the shipping address that we care about
	// because that's all we need for estimating the shipping cost.
	// The rest of the address will be collected by Stripe at checkout.
	Country string `json:"country"`
	// URL to redirect the user to after successful payment.
	SuccessURL string `json:"success_url"`
	// URL to redirect the user to if the press "Back" in the Stripe UI.
	CancelURL string `json:"cancel_url"`
	// User-provided promo code, if any.
	Promotion string `json:"promotion"`
	// Items in the cart.
	Items []Item `json:"items"`
}

type Item struct {
	// The Stripe Price ID.
	ID string `json:"id"`
	// How many units to buy.
	Qty uint8 `json:"qty"`
	// If the item is a bundle, its sub-products might have variants.
	Variants []string `json:"variants"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

// POST /checkout
//
// Create a new checkout session in Stripe and redirect the user there.
func checkoutOrder(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	pricesList, err := loadStripeList(client.V1Prices.List(r.Context(), &stripe.PriceListParams{
		Active: new(true),
	}))
	if err != nil {
		return ServerErrorR(r, "failed to list prices", err)
	}
	prices := make(map[string]*stripe.Price)
	for _, price := range pricesList {
		prices[price.ID] = price
	}
	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, len(attrs.Items))
	for i, item := range attrs.Items {
		price := prices[item.ID]
		if price == nil {
			return josh.BadRequest(josh.Error{
				Detail: fmt.Sprintf("price for item #%d not found", i+1),
			})
		}
		lineItem := stripe.CheckoutSessionCreateLineItemParams{
			Price:    &item.ID,
			Quantity: new(int64(item.Qty)),
			Metadata: map[string]string{},
		}
		if len(item.Variants) != 0 {
			for j, variantID := range item.Variants {
				price := prices[variantID]
				if price == nil {
					return josh.BadRequest(josh.Error{
						Detail: fmt.Sprintf("price for variant #%d of item #%d not found", j+1, i+1),
					})
				}
			}
			lineItem.Metadata["variants"] = strings.Join(item.Variants, ",")
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

// GET /order/{order}
//
// Get information about a specific order.
//
// Used by the order confirmation page.
func getOrder(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	orderID := r.PathValue("order")
	session, err := client.V1CheckoutSessions.Retrieve(r.Context(), orderID, &stripe.CheckoutSessionRetrieveParams{
		Expand: []*string{new("line_items")},
	})
	if err != nil {
		stripeErr, isStripeErr := err.(*stripe.Error)
		if isStripeErr && stripeErr.Code == "resource_missing" {
			return josh.NotFound(josh.Error{
				Detail: "order not found",
			})
		}
		return ServerErrorR(r, "failed to get order info", err)
	}
	if session.Customer.ID != string(customerID) {
		return josh.Forbidden(josh.Error{
			Detail: "you don't have access to the given order",
		})
	}
	return josh.Ok(formatOrder(session))
}

// GET /orders
//
// List all orders of the user.
func listOrders(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	customerID := josh.Must(josh.GetSingleton[CustomerID](r))

	sessions, err := loadStripeList(client.V1CheckoutSessions.List(r.Context(), &stripe.CheckoutSessionListParams{
		Customer: new(string(customerID)),
		Status:   new("complete"),
		Expand:   []*string{new("data.line_items")},
	}))
	if err != nil {
		return ServerErrorR(r, "failed to get order info", err)
	}
	resps := make([]josh.Data[Order], len(sessions))
	for i, session := range sessions {
		resps[i] = formatOrder(session)
	}
	return josh.Ok(resps)
}

func formatOrder(session *stripe.CheckoutSession) josh.Data[Order] {
	items := make([]OrderItem, 0, len(session.LineItems.Data))
	for _, item := range session.LineItems.Data {
		items = append(items, OrderItem{
			Name: item.Description,
			Qty:  item.Quantity,
		})
	}
	createdAt := time.Unix(session.Created, 0)
	id := ""
	if session.PaymentIntent != nil {
		id = session.PaymentIntent.ID[3:]
	}
	return josh.Data[Order]{
		ID:   id,
		Type: "order",
		Attributes: Order{
			Paid:      session.PaymentStatus == "paid",
			Amount:    session.AmountTotal,
			Currency:  string(session.Currency),
			CreatedAt: createdAt.Format(time.RFC3339),
			Items:     items,
		},
	}
}
