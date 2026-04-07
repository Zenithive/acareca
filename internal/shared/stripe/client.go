package stripe

import (
	"github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/price"
	"github.com/stripe/stripe-go/v82/product"
	sub "github.com/stripe/stripe-go/v82/subscription"
	"github.com/stripe/stripe-go/v82/webhook"
)

// StripeClient wraps all Stripe SDK calls behind an interface for testability.
type StripeClient interface {
	CreateProduct(name, description string) (productID string, err error)
	UpdateProduct(productID, name, description string) error
	SetDefaultPrice(productID, priceID string) error
	ArchiveProduct(productID string) error
	UnarchiveProduct(productID string) error

	CreatePrice(productID string, unitAmountCents int64, currency string) (priceID string, err error)
	ArchivePrice(priceID string) error

	CreateCustomer(email, name string) (customerID string, err error)

	CreateCheckoutSession(params *stripe.CheckoutSessionParams) (url string, err error)
	CreatePortalSession(customerID, returnURL string) (url string, err error)

	ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error)
	RetrieveSubscription(subID string) (*stripe.Subscription, error)
}

type stripeClient struct{}

// NewStripeClient returns a production StripeClient backed by the stripe-go SDK.
// The caller must set stripe.Key before invoking any methods.
func NewStripeClient() StripeClient {
	return &stripeClient{}
}

func (c *stripeClient) CreateProduct(name, description string) (string, error) {
	p, err := product.New(&stripe.ProductParams{
		Name:        stripe.String(name),
		Description: stripe.String(description),
	})
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func (c *stripeClient) UpdateProduct(productID, name, description string) error {
	_, err := product.Update(productID, &stripe.ProductParams{
		Name:        stripe.String(name),
		Description: stripe.String(description),
	})
	return err
}

func (c *stripeClient) SetDefaultPrice(productID, priceID string) error {
	_, err := product.Update(productID, &stripe.ProductParams{
		DefaultPrice: stripe.String(priceID),
	})
	return err
}

func (c *stripeClient) ArchiveProduct(productID string) error {
	_, err := product.Update(productID, &stripe.ProductParams{
		Active: stripe.Bool(false),
	})
	return err
}

func (c *stripeClient) UnarchiveProduct(productID string) error {
	_, err := product.Update(productID, &stripe.ProductParams{
		Active: stripe.Bool(true),
	})
	return err
}

func (c *stripeClient) CreatePrice(productID string, unitAmountCents int64, currency string) (string, error) {
	p, err := price.New(&stripe.PriceParams{
		Product:    stripe.String(productID),
		UnitAmount: stripe.Int64(unitAmountCents),
		Currency:   stripe.String(currency),
		Recurring: &stripe.PriceRecurringParams{
			Interval: stripe.String(string(stripe.PriceRecurringIntervalMonth)),
		},
	})
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func (c *stripeClient) ArchivePrice(priceID string) error {
	_, err := price.Update(priceID, &stripe.PriceParams{
		Active: stripe.Bool(false),
	})
	return err
}

func (c *stripeClient) CreateCustomer(email, name string) (string, error) {
	cust, err := customer.New(&stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	})
	if err != nil {
		return "", err
	}
	return cust.ID, nil
}

func (c *stripeClient) CreateCheckoutSession(params *stripe.CheckoutSessionParams) (string, error) {
	s, err := session.New(params)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (c *stripeClient) CreatePortalSession(customerID, returnURL string) (string, error) {
	s, err := portalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (c *stripeClient) ConstructWebhookEvent(payload []byte, sigHeader, secret string) (stripe.Event, error) {
	return webhook.ConstructEventWithOptions(payload, sigHeader, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
}

func (c *stripeClient) RetrieveSubscription(subID string) (*stripe.Subscription, error) {
	return sub.Get(subID, nil)
}
