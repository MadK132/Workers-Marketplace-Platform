package grpcserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"diploma/payment-service/internal/service"
)

type WebhookPaymentService interface {
	MarkPaymentCompletedByTransactionReference(ctx context.Context, transactionReference string) (service.PaymentResult, error)
	MarkPaymentFailedByTransactionReference(ctx context.Context, transactionReference string) (service.PaymentResult, error)
}

func NewWebhookHandler(payments WebhookPaymentService, webhookSecret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		if webhookSecret != "" && !validStripeSignature(body, r.Header.Get("Stripe-Signature"), webhookSecret) {
			http.Error(w, "invalid signature", http.StatusBadRequest)
			return
		}

		var event stripeEvent
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		sessionID := event.Data.Object.ID
		if sessionID == "" {
			http.Error(w, "missing checkout session id", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		switch event.Type {
		case "checkout.session.completed", "checkout.session.async_payment_succeeded":
			_, err = payments.MarkPaymentCompletedByTransactionReference(ctx, sessionID)
		case "checkout.session.async_payment_failed", "checkout.session.expired":
			_, err = payments.MarkPaymentFailedByTransactionReference(ctx, sessionID)
		default:
			w.WriteHeader(http.StatusOK)
			return
		}
		if err != nil {
			if errors.Is(err, service.ErrPaymentNotFound) {
				http.Error(w, "payment not found", http.StatusNotFound)
				return
			}
			http.Error(w, "payment update failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
	return mux
}

type stripeEvent struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID            string `json:"id"`
			PaymentStatus string `json:"payment_status"`
		} `json:"object"`
	} `json:"data"`
}

func validStripeSignature(payload []byte, header string, secret string) bool {
	timestamp, signatures := parseStripeSignature(header)
	if timestamp == "" || len(signatures) == 0 {
		return false
	}
	createdAt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	age := time.Since(time.Unix(createdAt, 0))
	if age > 5*time.Minute || age < -5*time.Minute {
		return false
	}

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := mac.Sum(nil)

	for _, signature := range signatures {
		actual, err := hex.DecodeString(signature)
		if err == nil && hmac.Equal(expected, actual) {
			return true
		}
	}
	return false
}

func parseStripeSignature(header string) (string, []string) {
	var timestamp string
	signatures := make([]string, 0)

	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}

	return timestamp, signatures
}
