package lib

import (
	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
)

// https://docs.stripe.com/tax/tax-codes
const (
	// Video Gaming Console - Portable
	taxConsole = "txcd_34022001"
	// Cash Donation
	taxDonation = "txcd_90000001"
	// Video Games - downloaded - non subscription - with permanent rights
	taxGame = "txcd_10201000"
	// Clothing & Footwear
	taxShirt = "txcd_30011000"
)

type OrderItem struct {
	ProductSlug dbtypes.ProductSlug `json:"product_slug"`
	ReleaseID   *dbtypes.ReleaseID  `json:"release_id"`
	Quantity    int32               `json:"quantity"`
	RetailPrice *int32              `json:"retail_price"`
	Fulfilled   bool                `json:"fulfilled"`
}

func addOrderItem(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	body, err := schemas.ReadBody[OrderItem](r, "order_item", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes
	if attrs.ProductSlug != "donation" {
		return josh.BadRequest(josh.Error{
			Detail: "only dontations are supported",
		})
	}

	// TODO(@orsinium): Lock the column to prevent adding items to non-draft orders.
	order, err := ensureOrder(r)
	if err != nil {
		return ServerErrorR(r, "failed to create order", err)
	}

	price := *attrs.RetailPrice
	item, err := queries.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
		Order:       order.ID,
		Product:     attrs.ProductSlug,
		Release:     nil,
		Quantity:    attrs.Quantity,
		RetailPrice: price,
	})
	if err != nil {
		return ServerErrorR(r, "failed to add product to the order", err)
	}

	return josh.Created(formatOrderItem(item))
}

func listOrderItems(r josh.Req) josh.Resp {
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	order, err := queries.GetDraftOrder(r.Context(), myID)
	if err == pgx.ErrNoRows {
		return josh.Ok([]any{})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get order", err)
	}

	items, err := queries.ListOrderItems(r.Context(), order.ID)
	if err != nil {
		return ServerErrorR(r, "failed to get products in the order", err)
	}

	resps := make([]josh.Data[OrderItem], 0, len(items))
	for _, item := range items {
		resps = append(resps, formatOrderItem(item))
	}
	return josh.Ok(resps)
}

// Get the draft order, creating it if it doesn't exist.
func ensureOrder(r josh.Req) (db.Order, error) {
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	order, err := queries.EnsureOrder(r.Context(), myID)
	if err != pgx.ErrNoRows {
		return order, err
	}
	return queries.GetDraftOrder(r.Context(), myID)
}

func formatOrderItem(item db.OrderItem) josh.Data[OrderItem] {
	return josh.Data[OrderItem]{
		ID:   formatID(item.ID),
		Type: "order_item",
		Attributes: OrderItem{
			ProductSlug: item.Product,
			ReleaseID:   item.Release,
			Quantity:    item.Quantity,
			RetailPrice: &item.RetailPrice,
			Fulfilled:   item.Fulfilled,
		},
	}
}
