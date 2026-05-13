package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type CreatePaymentInput struct {
	BookingID            int
	Amount               float64
	Currency             string
	TransactionReference string
}

type CreatePaymentResult struct {
	Provider             string
	TransactionReference string
	PaymentURL           string
}

type Provider interface {
	CreatePayment(ctx context.Context, input CreatePaymentInput) (CreatePaymentResult, error)
}

type Config struct {
	DefaultProvider          string
	CloudPaymentsPublicID    string
	CloudPaymentsCheckoutURL string
	KaspiMerchantID          string
	KaspiPaymentBaseURL      string
}

func New(cfg Config) Provider {
	return &CloudPaymentsKaspiProvider{cfg: cfg}
}

type CloudPaymentsKaspiProvider struct {
	cfg Config
}

func (p *CloudPaymentsKaspiProvider) CreatePayment(
	_ context.Context,
	input CreatePaymentInput,
) (CreatePaymentResult, error) {
	providerName := strings.TrimSpace(p.cfg.DefaultProvider)
	if providerName == "" {
		providerName = "cloudpayments_kaspi"
	}

	reference := input.TransactionReference
	if reference == "" {
		reference = fmt.Sprintf("%s_booking_%d", providerName, input.BookingID)
	}

	paymentURL := p.buildPaymentURL(input, reference)

	return CreatePaymentResult{
		Provider:             providerName,
		TransactionReference: reference,
		PaymentURL:           paymentURL,
	}, nil
}

func (p *CloudPaymentsKaspiProvider) buildPaymentURL(input CreatePaymentInput, reference string) string {
	if p.cfg.KaspiPaymentBaseURL != "" {
		values := url.Values{}
		values.Set("merchant_id", p.cfg.KaspiMerchantID)
		values.Set("invoice_id", reference)
		values.Set("amount", fmt.Sprintf("%.2f", input.Amount))
		values.Set("currency", input.Currency)
		return p.cfg.KaspiPaymentBaseURL + "?" + values.Encode()
	}

	if p.cfg.CloudPaymentsCheckoutURL != "" {
		values := url.Values{}
		values.Set("public_id", p.cfg.CloudPaymentsPublicID)
		values.Set("invoice_id", reference)
		values.Set("amount", fmt.Sprintf("%.2f", input.Amount))
		values.Set("currency", input.Currency)
		return p.cfg.CloudPaymentsCheckoutURL + "?" + values.Encode()
	}

	return ""
}
