package lib

import (
	"strings"

	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/firefly-zero/api.fireflyzero.com/lib/schemas"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
)

type OrderItem struct {
	Name        string              `json:"name"`
	ProductSlug dbtypes.ProductSlug `json:"product_slug"`
	ReleaseID   *dbtypes.ReleaseID  `json:"release_id"`
	Quantity    int32               `json:"quantity"`
	RetailPrice *int32              `json:"retail_price"`
	Fulfilled   bool                `json:"fulfilled"`
}

type OrderItemPatch struct {
	Quantity    *int32 `json:"quantity"`
	RetailPrice *int32 `json:"retail_price"`
}

func withOrderItem(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		queries := josh.Must(josh.GetSingleton[*db.Queries](r))
		order := josh.Must(josh.GetSingleton[db.Order](r))

		itemID, errResp := josh.GetID[dbtypes.OrderItemID](r, "item")
		if errResp != nil {
			return josh.BadRequest(*errResp)
		}
		params := db.GetOrderItemParams{
			Order: order.ID,
			ID:    itemID,
		}
		item, err := queries.GetOrderItem(r.Context(), params)
		if err == pgx.ErrNoRows {
			return josh.NotFound(josh.Error{
				Detail: "order item not found",
				Code:   "order-item-404",
			})
		}
		if err != nil {
			return ServerErrorR(r, "failed to get order item", err)
		}
		r = josh.Must(josh.WithSingleton(r, item))
		return h(r)
	}
}

// POST /order/items
//
// Add an item into the pending order.
func addOrderItem(r josh.Req) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	body, err := schemas.ReadBody[OrderItem](r, "order_item", "_add")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	isApp := strings.Contains(string(attrs.ProductSlug), ".")
	if isApp && attrs.Quantity != 1 {
		return josh.BadRequest(josh.Error{
			Detail: "you can buy only one copy of an app",
		})
	}
	product, err := queries.GetProduct(r.Context(), attrs.ProductSlug)
	if err == pgx.ErrNoRows {
		_, err := queries.GetGroup(r.Context(), string(product.Slug))
		if err == nil {
			return josh.BadRequest(josh.Error{
				Detail: "group slug cannot be used as product slug",
			})
		}
		return josh.BadRequest(josh.Error{
			Detail: "product not found",
		})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get product info", err)
	}

	// TODO(@orsinium): Lock the column to prevent adding items to non-draft orders.
	order, err := ensureOrder(r)
	if err != nil {
		return ServerErrorR(r, "failed to create order", err)
	}

	price := product.RetailPrice
	if product.Slug == "donation" {
		if attrs.RetailPrice == nil {
			price = 5_00
		} else {
			price = *attrs.RetailPrice
		}
	}
	qty := attrs.Quantity
	// Quantity is not specified in the request for donations.
	if qty == 0 {
		qty = 1
	}
	item, err := queries.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
		Order:       order.ID,
		Product:     attrs.ProductSlug,
		Release:     nil,
		Quantity:    qty,
		RetailPrice: price,
	})
	if err != nil {
		return ServerErrorR(r, "failed to add product to the order", err)
	}

	return josh.Created(formatOrderItem(item, product))
}

// GET /order/items
//
// List items in the draft order.
func listDraftOrderItems(r josh.Req) josh.Resp {
	myID := josh.Must(josh.GetSingleton[dbtypes.UserID](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	order, err := queries.GetDraftOrder(r.Context(), myID)
	if err == pgx.ErrNoRows {
		return josh.Ok([]any{})
	}
	if err != nil {
		return ServerErrorR(r, "failed to get order", err)
	}
	return listItemsInOrder(r, order)
}

// GET /order/{order}/items
//
// List items in the given order.
func listOrderItems(r josh.Req) josh.Resp {
	order := josh.Must(josh.GetSingleton[db.Order](r))

	if order.Status == db.OrderStatusDraft {
		return josh.BadRequest(josh.Error{
			Detail: "order is in draft",
		})
	}
	return listItemsInOrder(r, order)
}

func listItemsInOrder(r josh.Req, order db.Order) josh.Resp {
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	items, err := queries.ListOrderItems(r.Context(), order.ID)
	if err != nil {
		return ServerErrorR(r, "failed to get products in the order", err)
	}

	resps := make([]josh.Data[OrderItem], 0, len(items))
	for _, item := range items {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			// No return, it's ok'ish to return
			// the order item without product info.
			ServerErrorR(r, "cannot find product", err)
		}
		resps = append(resps, formatOrderItem(item, product))
	}
	return josh.Ok(resps)
}

func patchOrderItem(r josh.Req) josh.Resp {
	order := josh.Must(josh.GetSingleton[db.Order](r))

	body, err := schemas.ReadBody[OrderItemPatch](r, "order_item", "_patch")
	if err != nil {
		return josh.BadRequest(josh.Error{Detail: err.Error()})
	}
	attrs := body.Attributes

	if order.Status != db.OrderStatusDraft {
		return josh.BadRequest(josh.Error{
			Detail: "order is not in draft",
		})
	}

	if attrs.Quantity != nil {
		return adjustQuantity(r, *attrs.Quantity)
	}
	if attrs.RetailPrice != nil {
		return adjustRetailPrice(r, *attrs.RetailPrice)
	}
	return josh.NotModified()
}

func adjustQuantity(r josh.Req, qty int32) josh.Resp {
	order := josh.Must(josh.GetSingleton[db.Order](r))
	item := josh.Must(josh.GetSingleton[db.OrderItem](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	isApp := strings.Contains(string(item.Product), ".")
	if isApp {
		return josh.BadRequest(josh.Error{
			Detail: "you can buy only one app",
		})
	}
	if item.Product == "donation" {
		return josh.BadRequest(josh.Error{
			Detail: "you can adjust the amount of donation but not quantity",
		})
	}
	if qty == 0 {
		err := queries.DeleteOrderItem(r.Context(), db.DeleteOrderItemParams{
			Order: order.ID,
			ID:    item.ID,
		})
		if err != nil {
			return ServerErrorR(r, "failed to delete order item", err)
		}
		return josh.NoContent()
	}
	if qty != item.Quantity {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			ServerErrorR(r, "cannot find product", err)
		}
		params := db.SetOrderItemQuantityParams{
			Quantity: qty,
			Order:    order.ID,
			ID:       item.ID,
		}
		item, err := queries.SetOrderItemQuantity(r.Context(), params)
		if err != nil {
			ServerErrorR(r, "failed to adjust order item quantity", err)
		}
		return josh.Ok(formatOrderItem(item, product))
	}
	return josh.NotModified()
}

func adjustRetailPrice(r josh.Req, price int32) josh.Resp {
	order := josh.Must(josh.GetSingleton[db.Order](r))
	item := josh.Must(josh.GetSingleton[db.OrderItem](r))
	queries := josh.Must(josh.GetSingleton[*db.Queries](r))

	if item.Product == "donation" {
		return josh.BadRequest(josh.Error{
			Detail: "you can adjust the price only for donations",
		})
	}

	if price == 0 {
		err := queries.DeleteOrderItem(r.Context(), db.DeleteOrderItemParams{
			Order: order.ID,
			ID:    item.ID,
		})
		if err != nil {
			return ServerErrorR(r, "failed to delete order item", err)
		}
		return josh.NoContent()
	}
	if price != item.RetailPrice {
		product, err := queries.GetProduct(r.Context(), item.Product)
		if err != nil {
			ServerErrorR(r, "cannot find product", err)
		}
		params := db.SetOrderItemRetailPriceParams{
			RetailPrice: price,
			Order:       order.ID,
			ID:          item.ID,
		}
		item, err := queries.SetOrderItemRetailPrice(r.Context(), params)
		if err != nil {
			ServerErrorR(r, "failed to adjust order item quantity", err)
		}
		return josh.Ok(formatOrderItem(item, product))
	}
	return josh.NotModified()

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

func formatOrderItem(item db.OrderItem, product db.Product) josh.Data[OrderItem] {
	return josh.Data[OrderItem]{
		ID:   formatID(item.ID),
		Type: "order_item",
		Attributes: OrderItem{
			Name:        product.Name,
			ProductSlug: item.Product,
			ReleaseID:   item.Release,
			Quantity:    item.Quantity,
			RetailPrice: &item.RetailPrice,
			Fulfilled:   item.Fulfilled,
		},
	}
}
