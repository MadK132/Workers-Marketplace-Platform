package client

import (
	"context"
	"time"

	paymentpb "diploma/api/payment-service-proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type PaymentClient struct {
	address       string
	gatewaySecret string
	provider      string
	currency      string
}

type PaymentResult struct {
	PaymentID int64  `json:"payment_id"`
	Status    string `json:"payment_status"`
	URL       string `json:"payment_url"`
}

func NewPaymentClient(address string, gatewaySecret string, provider string, currency string) *PaymentClient {
	return &PaymentClient{
		address:       address,
		gatewaySecret: gatewaySecret,
		provider:      provider,
		currency:      currency,
	}
}

func (c *PaymentClient) CreatePayment(ctx context.Context, bookingID int, amount float64) (PaymentResult, error) {
	callCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if c.gatewaySecret != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-gateway-secret", c.gatewaySecret)
	}

	conn, err := grpc.NewClient(c.address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return PaymentResult{}, err
	}
	defer conn.Close()

	resp, err := paymentpb.NewPaymentServiceClient(conn).CreatePayment(callCtx, &paymentpb.CreatePaymentRequest{
		BookingId: int64(bookingID),
		Amount:    amount,
		Currency:  c.currency,
		Provider:  c.provider,
	})
	if err != nil {
		return PaymentResult{}, err
	}

	return PaymentResult{
		PaymentID: resp.GetPaymentId(),
		Status:    resp.GetStatus().String(),
		URL:       resp.GetPaymentUrl(),
	}, nil
}
