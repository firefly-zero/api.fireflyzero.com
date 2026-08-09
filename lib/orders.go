package lib

import (
	"context"
	"strings"
	"time"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Order struct {
	Paid bool `json:"paid"`
}

type CheckoutReq struct {
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

type CheckoutResp struct {
	RedirectURL string `json:"redirect_url"`
}

type ConfirmReq struct {
	SessionID string `json:"session_id"`
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

func checkoutOrder(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))

	body, err := schemas.ReadBody[CheckoutReq](r, "checkout", "_req")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	me, err := queries.GetUserByID(r.Context(), myID)
	if err != nil {
		return ServerErrorR(r, "failed to get user info", err)
	}
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
			Quantity: new(int64(item.Quantity)),
			PriceData: &stripe.CheckoutSessionCreateLineItemPriceDataParams{
				Currency:    new(string(stripe.CurrencyEUR)),
				ProductData: new(makeProductData(product)),
				TaxBehavior: new(string),
				UnitAmount:  new(int64(item.RetailPrice)),
			},
		}
		lineItems = append(lineItems, &lineItem)
	}

	// From this point on, the request cannot be canceled.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()

	orderIDStr := formatID(order.ID)
	params := stripe.CheckoutSessionCreateParams{
		Params: stripe.Params{
			IdempotencyKey: new("order-checkout-" + orderIDStr),
		},
		Mode:       new(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: new(formatOrderURL(attrs.SuccessURL, order.ID)),
		CancelURL:  new(formatOrderURL(attrs.CancelURL, order.ID)),
		Customer:   &me.StripeID,
		LineItems:  lineItems,
		Metadata: map[string]string{
			"order_id": orderIDStr,
		},
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

	_, err = queries.SetOrderStatus(ctx, db.SetOrderStatusParams{
		Status: db.OrderStatusPending,
		ID:     order.ID,
		User:   myID,
	})
	if err != nil {
		return ServerErrorR(r, "failed to update order status", err)
	}

	return josh.Ok(josh.Data[CheckoutResp]{
		ID:   orderIDStr,
		Type: "checkout",
		Attributes: CheckoutResp{
			RedirectURL: session.URL,
		},
	})
}

func formatOrderURL(template string, id dbtypes.OrderID) string {
	return strings.ReplaceAll(template, "{order}", formatID(id))
}

func makeProductData(p db.Product) stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams {
	return stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
		Metadata: map[string]string{
			"product_slug": string(p.Slug),
		},
		Name:    &p.Name,
		TaxCode: &p.TaxCode,
	}
}

func formatOrder(order db.Order) josh.Data[Order] {
	return josh.Data[Order]{
		ID:   formatID(order.ID),
		Type: "order",
		Attributes: Order{
			Paid: order.Paid,
		},
	}
}
