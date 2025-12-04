package lib

import (
	"github.com/firefly-zero/api.fireflyzero.com/lib/db"
	"github.com/firefly-zero/api.fireflyzero.com/lib/dbtypes"
	"github.com/jackc/pgx/v5"
	"github.com/orsinium-labs/josh"
)

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
