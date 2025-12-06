package lib

import (
	"strings"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type CheckoutReq struct {
	SuccessURL string `json:"success_url"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

func withOrder(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		queries := josh.Must(josh.GetSingleton[*db.Queries](r))
		myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

		orderID, errResp := josh.GetID[dbtypes.OrderID](r, "order")
		if errResp != nil {
			return josh.BadRequest(*errResp)
		}
		params := db.GetOrderParams{
			User: myID,
			ID:   orderID,
		}
		order, err := queries.GetOrder(r.Context(), params)
		if err == pgx.ErrNoRows {
			return josh.NotFound(josh.Error{
				Detail: "order not found",
				Code:   "order-404",
			})
		}
		if err != nil {
			return ServerErrorR(r, "failed to get order", err)
		}
		r = josh.Must(josh.WithSingleton(r, order))
		return h(r)
	}
}

func checkout(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	order, err := queries.GetDraftOrder(r.Context(), myID)
	if err != nil {
		return ServerErrorR(r, "failed to get order", err)
	}
	if order.Status != db.OrderStatusDraft {
		return josh.BadRequest(josh.Error{
			Detail: "you can checkout only draft order",
		})
	}
	items, err := queries.ListOrderItems(r.Context(), order.ID)
	if err != nil {
		return ServerErrorR(r, "failed to list order items", err)
	}

	lineItems := make([]*stripe.CheckoutSessionCreateLineItemParams, 0, len(items))
	for _, item := range items {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			return ServerErrorR(r, "failed to get product info", err)
		}
		lineItem := stripe.CheckoutSessionCreateLineItemParams{
			Quantity: ptr(int64(item.Quantity)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    ptr(string(stripe.CurrencyEUR)),
				ProductData: ptr(makeProductData(product)),
				TaxBehavior: new(string),
				UnitAmount:  ptr(int64(item.RetailPrice)),
			},
		}
		lineItems = append(lineItems, &lineItem)
	}

	successURL := attrs.SuccessURL + "?order=" + formatID(order.ID)
	params := stripe.CheckoutSessionCreateParams{
		Mode:       ptr(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: &successURL,
		LineItems:  lineItems,
	}
	session, err := client.V1CheckoutSessions.Create(r.Context(), &params)
	if err != nil {
		return ServerErrorR(r, "failed to create checkout session", err)
	}
	return josh.Ok(josh.Data[CheckoutResp]{
		ID:   formatID(order.ID),
		Type: "checkout",
		Attributes: CheckoutResp{
			RedirectURL: session.URL,
		},
	})
}

func makeProductData(p db.Product) stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams {
	return stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
		Metadata: map[string]string{
			"product_slug": string(p.Slug),
		},
		Name:    &p.Name,
		TaxCode: ptr(getTaxCode(p.Slug)),
	}
}

// https://docs.stripe.com/tax/tax-codes
func getTaxCode(slug dbtypes.ProductSlug) string {
	isApp := strings.Contains(string(slug), ".")
	if isApp {
		// Video Games - downloaded - non subscription - with permanent rights
		return "txcd_10201000"
	}
	switch slug {
	case "donation":
		// Cash Donation
		return "txcd_90000001"
	case "device", "firefly", "firefly-zero":
		// Video Gaming Console - Portable
		return "txcd_34022001"
	default:
		// Clothing & Footwear
		return "txcd_30011000"
	}
}

func ptr[T any](v T) *T {
	return &v
}
