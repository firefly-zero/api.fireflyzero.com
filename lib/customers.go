package lib

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/orsinium-labs/josh"
	"github.com/stripe/stripe-go/v84"
)

type CustomerID string

var customerIDs = SafeMap[string, CustomerID]{}

func withCustomer(h josh.Handler) josh.Handler {
	return func(r josh.Req) josh.Resp {
		customerID, err := ensureCustomerID(r)
		if err != nil {
			return ServerErrorR(r, "failed to fetch customer info", err)
		}
		r = josh.Must(josh.WithSingleton(r, customerID))
		return h(r)
	}
}

func ensureCustomerID(r josh.Req) (CustomerID, error) {
	jwt := josh.Must(josh.GetSingleton[JWT](r))

	if jwt.CustomerID != "" {
		return jwt.CustomerID, nil
	}
	id, found := customerIDs.Get(jwt.Email)
	if found {
		return id, nil
	}
	customer, err := ensureCustomer(r)
	if err != nil {
		return "", err
	}
	id = CustomerID(customer.ID)
	customerIDs.Set(jwt.Email, id)
	return id, err
}

func ensureCustomer(r josh.Req) (*stripe.Customer, error) {
	jwt := josh.Must(josh.GetSingleton[JWT](r))
	client := josh.Must(josh.GetSingleton[*stripe.Client](r))

	// Find customer.
	{
		if strings.Contains(jwt.Email, "'") {
			return nil, errors.New("invalid email")
		}
		customers, err := loadStripeList(client.V1Customers.List(r.Context(), &stripe.CustomerListParams{
			Email: &jwt.Email,
		}))
		if err != nil {
			return nil, fmt.Errorf("search customer: %w", err)
		}
		if len(customers) > 1 {
			slices.SortFunc(customers, func(a, b *stripe.Customer) int {
				return cmp.Compare(a.Created, b.Created)
			})
			for _, customer := range customers[1:] {
				_, _ = client.V1Customers.Delete(r.Context(), customer.ID, nil)
			}
		}
		if len(customers) != 0 {
			return customers[0], nil
		}
	}

	// Create customer.
	customer, err := client.V1Customers.Create(r.Context(), &stripe.CustomerCreateParams{
		Email: &jwt.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("create customer: %w", err)
	}
	return customer, nil
}

type SafeMap[K comparable, V any] struct {
	items map[K]V
	mx    sync.Mutex
}

func (m *SafeMap[K, V]) Get(k K) (V, bool) {
	m.mx.Lock()
	if m.items == nil {
		m.items = make(map[K]V, 0)
	}
	v, found := m.items[k]
	m.mx.Unlock()
	return v, found
}

func (m *SafeMap[K, V]) Set(k K, v V) {
	m.mx.Lock()
	if m.items == nil {
		m.items = make(map[K]V, 0)
	}
	m.items[k] = v
	m.mx.Unlock()
}
