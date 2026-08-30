package lib

import (
	"cmp"
	"slices"
	"strings"

	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

// The order in which t-shirt sizes should be listed.
var sizes = []string{
	"XXXS",
	"XXS",
	"XS",
	"S",
	"M",
	"L",
	"XL",
	"XXL",
	"XXXL",
}

type Product struct {
	// Human-readable unique title of the product.
	Name string `json:"name"`

	// Human-readable (but not intended to be displayed) unique reference to the product.
	//
	// Used to detect donations (which are displayed differently in the UI).
	Slug string `json:"slug"`

	// Human-readable plaintext short description of the product.
	Description string `json:"description"`

	// URL pointing to the cover image for the product.
	//
	// Hosted on Stripe CDN. Can have any ratio.
	// It's up to the client (frontend) to crop it to the right size.
	Image *string `json:"image"`

	// If true, the product is out of stock and cannot be purchased.
	//
	// However, it still should be displayed.
	OutOfStock bool `json:"out_of_stock"`

	// Badge message to display, like "limited edition".
	Badge *string `json:"badge"`

	// Display with icon on the badge.
	BadgeIcon *string `json:"badge_icon"`

	// Product variants, like different t-shirt sizes.
	//
	// Every product has at least one variant.
	Variants []josh.Data[Variant] `json:"variants"`

	// If the product is a bundle, here will be the list of references
	// to the products in that bundle.
	Products []BundleProduct `json:"products"`
}

// Reference to a sub-product in a bundle.
type BundleProduct struct {
	Slug string `json:"slug"`
	Qty  int32  `json:"qty"`
}

// A product variant, like a specific size of a t-shirt.
//
// Represented as Price in Stripe. Every Product has at least one Variant.
type Variant struct {
	// The human-readable name of the variant.
	//
	// Usually empty for products with a single variant.
	// Never empty for products with multiple variants.
	Name string `json:"name"`
	// How much it costs in euros.
	Price int32 `json:"price"`
}

// GET /products
//
// List all products currently available for sale
// (including products only available in a bundle).
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

	slices.SortStableFunc(products, cmpProducts)
	resps := make([]josh.Data[Product], len(products))
	for i, product := range products {
		productPrices := filterProductPrices(prices, product)
		resps[i] = formatProduct(product, productPrices)
	}
	return josh.Ok(resps)
}

func cmpProducts(a, b *stripe.Product) int {
	// If both products have position set, sort by the position.
	aPos := a.Metadata["position"]
	bPos := a.Metadata["position"]
	if aPos != bPos {
		return cmp.Compare(aPos, bPos)
	}

	// Bundles go next.
	aBundle := a.Metadata["products"] != ""
	bBundle := b.Metadata["products"] != ""
	if aBundle != bBundle {
		if aBundle {
			return -1
		} else {
			return 1
		}
	}

	return cmp.Compare(a.Name, b.Name)
}

// Get the sorted list of prices for the given product.
func filterProductPrices(prices []*stripe.Price, product *stripe.Product) []*stripe.Price {
	productPrices := make([]*stripe.Price, 0)
	for _, price := range prices {
		if price.Product.ID == product.ID {
			productPrices = append(productPrices, price)
		}
	}
	slices.SortStableFunc(productPrices, func(a, b *stripe.Price) int {
		aIdx := slices.Index(sizes, a.LookupKey)
		bIdx := slices.Index(sizes, b.LookupKey)
		if aIdx != -1 && bIdx != -1 {
			return cmp.Compare(aIdx, bIdx)
		}
		return cmp.Compare(a.UnitAmount, b.UnitAmount)
	})
	return productPrices
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
	var image *string
	if len(p.Images) != 0 {
		image = &p.Images[0]
	}
	var badge *string
	if p.Metadata["badge"] != "" {
		badge = new(p.Metadata["badge"])
	}
	var badgeIcon *string
	if p.Metadata["badge-icon"] != "" {
		badgeIcon = new(p.Metadata["badge-icon"])
	}
	products := []BundleProduct{}
	for slug := range strings.SplitSeq(p.Metadata["products"], ",") {
		slug = strings.TrimSpace(slug)
		if slug != "" {
			if len(products) > 0 && products[len(products)-1].Slug == slug {
				products[len(products)-1].Qty++
			} else {
				bundleProduct := BundleProduct{
					Slug: slug,
					Qty:  1,
				}
				products = append(products, bundleProduct)
			}
		}
	}
	return josh.Data[Product]{
		ID:   p.ID,
		Type: "product",
		Attributes: Product{
			Name:        p.Name,
			Slug:        p.Metadata["slug"],
			Description: p.Description,
			Image:       image,
			Badge:       badge,
			BadgeIcon:   badgeIcon,
			OutOfStock:  p.Metadata["out-of-stock"] == "true",
			Variants:    variants,
			Products:    products,
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
