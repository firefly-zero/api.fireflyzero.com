package lib

import (
	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type Product struct {
	Name        string
	Description string
	Variants    []josh.Data[Variant]
}

type Variant struct {
	Name  string
	Price int32
}

func listProducts(r josh.Req) josh.Resp {
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))

	products, err := loadStripeList(client.V1Products.List(r.Context(), &stripe.ProductListParams{
		Active: new(true),
	}))
	if err != nil {
		return ServerErrorR(r, "failed to retrieve the list of products", err)
	}
	prices, err := loadStripeList(client.V1Prices.List(r.Context(), &stripe.PriceListParams{
		ListParams: stripe.ListParams{},
		Active:     new(true),
	}))
	if err != nil {
		return ServerErrorR(r, "failed to retrieve the list of prices", err)
	}

	resps := make([]josh.Data[Product], len(products))
	for i, product := range products {
		productPrices := make([]*stripe.Price, 0)
		for _, price := range prices {
			if price.Product.ID == product.ID {
				productPrices = append(productPrices, price)
			}
		}
		resps[i] = formatProduct(product, productPrices)
	}
	return josh.Ok(resps)
}

func formatProduct(p *stripe.Product, prices []*stripe.Price) josh.Data[Product] {
	variants := make([]josh.Data[Variant], len(prices))
	for i, price := range prices {
		variants[i] = josh.Data[Variant]{
			ID:   price.ID,
			Type: "price",
			Attributes: Variant{
				Name:  price.LookupKey,
				Price: int32(price.UnitAmount),
			},
		}
	}
	return josh.Data[Product]{
		ID:   p.ID,
		Type: "product",
		Attributes: Product{
			Name:        p.Name,
			Description: p.Description,
			Variants:    variants,
		},
	}
}

func loadStripeList[T any](seq stripe.Seq2[T, error]) ([]T, error) {
	res := make([]T, 0)
	for val, err := range seq {
		if err != nil {
			return nil, err
		}
		res = append(res, val)
	}
	return res, nil
}
