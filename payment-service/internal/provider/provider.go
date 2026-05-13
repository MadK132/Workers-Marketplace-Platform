package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type CreatePaymentInput struct {
	BookingID            int
	Amount               float64
	Currency             string
	Provider             string
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
	DefaultProvider      string
	StripeSecretKey      string
	StripeSuccessURL     string
	StripeCancelURL      string
	StripeCheckoutAPIURL string
	StripeMockURL        string
}

func New(cfg Config) Provider {
	if cfg.StripeCheckoutAPIURL == "" {
		cfg.StripeCheckoutAPIURL = "https://api.stripe.com/v1/checkout/sessions"
	}
	if cfg.StripeMockURL == "" {
		cfg.StripeMockURL = "http://localhost:5173/payment/mock"
	}
	return &StripeProvider{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type StripeProvider struct {
	cfg    Config
	client *http.Client
}

type stripeSessionResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type stripeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *StripeProvider) CreatePayment(
	ctx context.Context,
	input CreatePaymentInput,
) (CreatePaymentResult, error) {
	providerName := strings.TrimSpace(input.Provider)
	if providerName == "" {
		providerName = strings.TrimSpace(p.cfg.DefaultProvider)
	}
	if providerName == "" {
		providerName = "stripe"
	}

	reference := input.TransactionReference
	if reference == "" {
		reference = fmt.Sprintf("%s_booking_%d", providerName, input.BookingID)
	}

	if strings.TrimSpace(p.cfg.StripeSecretKey) == "" {
		return CreatePaymentResult{
			Provider:             providerName,
			TransactionReference: reference,
			PaymentURL:           p.buildMockPaymentURL(input, reference),
		}, nil
	}

	session, err := p.createCheckoutSession(ctx, input, reference)
	if err != nil {
		return CreatePaymentResult{}, err
	}

	transactionReference := reference
	if session.ID != "" {
		transactionReference = session.ID
	}

	return CreatePaymentResult{
		Provider:             providerName,
		TransactionReference: transactionReference,
		PaymentURL:           session.URL,
	}, nil
}

func (p *StripeProvider) createCheckoutSession(
	ctx context.Context,
	input CreatePaymentInput,
	reference string,
) (stripeSessionResponse, error) {
	values := url.Values{}
	values.Set("mode", "payment")
	values.Set("success_url", p.cfg.StripeSuccessURL)
	values.Set("cancel_url", p.cfg.StripeCancelURL)
	values.Set("client_reference_id", reference)
	values.Set("line_items[0][quantity]", "1")
	values.Set("line_items[0][price_data][currency]", strings.ToLower(input.Currency))
	values.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(toMinorUnits(input.Amount), 10))
	values.Set("line_items[0][price_data][product_data][name]", fmt.Sprintf("Booking #%d", input.BookingID))
	values.Set("metadata[booking_id]", strconv.Itoa(input.BookingID))
	values.Set("metadata[transaction_reference]", reference)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.cfg.StripeCheckoutAPIURL,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return stripeSessionResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Idempotency-Key", reference)
	req.SetBasicAuth(p.cfg.StripeSecretKey, "")

	resp, err := p.client.Do(req)
	if err != nil {
		return stripeSessionResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return stripeSessionResponse{}, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return stripeSessionResponse{}, stripeError(resp.StatusCode, body)
	}

	var session stripeSessionResponse
	if err := json.Unmarshal(body, &session); err != nil {
		return stripeSessionResponse{}, err
	}
	if session.URL == "" {
		return stripeSessionResponse{}, fmt.Errorf("stripe checkout session response has empty url")
	}

	return session, nil
}

func (p *StripeProvider) buildMockPaymentURL(input CreatePaymentInput, reference string) string {
	values := url.Values{}
	values.Set("booking_id", strconv.Itoa(input.BookingID))
	values.Set("amount", fmt.Sprintf("%.2f", input.Amount))
	values.Set("currency", input.Currency)
	values.Set("reference", reference)
	return p.cfg.StripeMockURL + "?" + values.Encode()
}

func stripeError(statusCode int, body []byte) error {
	var stripeErr stripeErrorResponse
	if err := json.Unmarshal(body, &stripeErr); err == nil && stripeErr.Error.Message != "" {
		return fmt.Errorf("stripe checkout session error: status=%d type=%s message=%s", statusCode, stripeErr.Error.Type, stripeErr.Error.Message)
	}
	return fmt.Errorf("stripe checkout session error: status=%d body=%s", statusCode, strings.TrimSpace(string(body)))
}

func toMinorUnits(amount float64) int64 {
	return int64(math.Round(amount * 100))
}
