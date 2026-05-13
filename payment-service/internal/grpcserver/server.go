package grpcserver

import (
	"context"
	"errors"

	paymentpb "diploma/api/payment-service-proto"
	"diploma/payment-service/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentService interface {
	CreatePayment(ctx context.Context, bookingID int, amount float64, currency string, provider string) (service.PaymentResult, error)
	GetPayment(ctx context.Context, paymentID int) (service.PaymentResult, error)
	MarkPaymentCompleted(ctx context.Context, paymentID int, transactionReference string) (service.PaymentResult, error)
	MarkPaymentFailed(ctx context.Context, paymentID int, transactionReference string) (service.PaymentResult, error)
}

type Server struct {
	paymentpb.UnimplementedPaymentServiceServer
	payments PaymentService
}

func New(payments PaymentService) *Server {
	return &Server{payments: payments}
}

func (s *Server) CreatePayment(
	ctx context.Context,
	req *paymentpb.CreatePaymentRequest,
) (*paymentpb.PaymentResponse, error) {
	result, err := s.payments.CreatePayment(
		ctx,
		int(req.GetBookingId()),
		req.GetAmount(),
		req.GetCurrency(),
		req.GetProvider(),
	)
	if err != nil {
		return nil, grpcError(err)
	}

	return paymentResponse(result), nil
}

func (s *Server) GetPayment(
	ctx context.Context,
	req *paymentpb.GetPaymentRequest,
) (*paymentpb.PaymentResponse, error) {
	result, err := s.payments.GetPayment(ctx, int(req.GetPaymentId()))
	if err != nil {
		return nil, grpcError(err)
	}

	return paymentResponse(result), nil
}

func (s *Server) MarkPaymentCompleted(
	ctx context.Context,
	req *paymentpb.UpdatePaymentStatusRequest,
) (*paymentpb.PaymentResponse, error) {
	result, err := s.payments.MarkPaymentCompleted(
		ctx,
		int(req.GetPaymentId()),
		req.GetTransactionReference(),
	)
	if err != nil {
		return nil, grpcError(err)
	}

	return paymentResponse(result), nil
}

func (s *Server) MarkPaymentFailed(
	ctx context.Context,
	req *paymentpb.UpdatePaymentStatusRequest,
) (*paymentpb.PaymentResponse, error) {
	result, err := s.payments.MarkPaymentFailed(
		ctx,
		int(req.GetPaymentId()),
		req.GetTransactionReference(),
	)
	if err != nil {
		return nil, grpcError(err)
	}

	return paymentResponse(result), nil
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidPaymentInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrPaymentNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func paymentResponse(result service.PaymentResult) *paymentpb.PaymentResponse {
	payment := result.Payment
	return &paymentpb.PaymentResponse{
		PaymentId:            int64(payment.ID),
		BookingId:            int64(payment.BookingID),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		Status:               paymentStatus(payment.Status),
		Provider:             payment.Provider,
		TransactionReference: payment.TransactionReference,
		PaymentUrl:           result.PaymentURL,
	}
}

func paymentStatus(value string) paymentpb.PaymentStatus {
	switch value {
	case "pending":
		return paymentpb.PaymentStatus_PAYMENT_STATUS_PENDING
	case "completed":
		return paymentpb.PaymentStatus_PAYMENT_STATUS_COMPLETED
	case "failed":
		return paymentpb.PaymentStatus_PAYMENT_STATUS_FAILED
	case "refunded":
		return paymentpb.PaymentStatus_PAYMENT_STATUS_REFUNDED
	default:
		return paymentpb.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	}
}
