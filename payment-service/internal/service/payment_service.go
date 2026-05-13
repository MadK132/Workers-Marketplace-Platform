package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"diploma/payment-service/internal/provider"
	"diploma/payment-service/internal/repository"
)

var (
	ErrInvalidPaymentInput = errors.New("invalid payment input")
	ErrPaymentNotFound     = repository.ErrPaymentNotFound
)

type PaymentRepository interface {
	Create(ctx context.Context, bookingID int, amount float64, currency string, provider string, transactionReference string) (repository.Payment, error)
	GetByBookingID(ctx context.Context, bookingID int) (repository.Payment, error)
	GetByID(ctx context.Context, paymentID int) (repository.Payment, error)
	UpdateStatus(ctx context.Context, paymentID int, status string, transactionReference string) (repository.Payment, error)
	UpdateStatusByTransactionReference(ctx context.Context, transactionReference string, status string) (repository.Payment, error)
}

type PaymentProvider interface {
	CreatePayment(ctx context.Context, input provider.CreatePaymentInput) (provider.CreatePaymentResult, error)
}

type PaymentResult struct {
	Payment    repository.Payment
	PaymentURL string
}

type PaymentService struct {
	repo     PaymentRepository
	provider PaymentProvider
}

func NewPaymentService(repo PaymentRepository, provider PaymentProvider) *PaymentService {
	return &PaymentService{
		repo:     repo,
		provider: provider,
	}
}

func (s *PaymentService) CreatePayment(
	ctx context.Context,
	bookingID int,
	amount float64,
	currency string,
	providerName string,
) (PaymentResult, error) {
	if bookingID <= 0 || amount <= 0 {
		return PaymentResult{}, ErrInvalidPaymentInput
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "KZT"
	}
	if len(currency) != 3 {
		return PaymentResult{}, fmt.Errorf("%w: currency must be ISO-4217 code", ErrInvalidPaymentInput)
	}
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		providerName = "stripe"
	}

	reference := fmt.Sprintf("%s_booking_%d", providerName, bookingID)
	var payment repository.Payment
	existing, err := s.repo.GetByBookingID(ctx, bookingID)
	if err == nil {
		if existing.Status != "failed" {
			return PaymentResult{Payment: existing}, nil
		}
		payment = existing
	}
	if !errors.Is(err, repository.ErrPaymentNotFound) {
		return PaymentResult{}, err
	}

	if payment.ID == 0 {
		payment, err = s.repo.Create(
			ctx,
			bookingID,
			amount,
			currency,
			providerName,
			reference,
		)
		if err != nil {
			return PaymentResult{}, err
		}
	}

	providerResult, err := s.provider.CreatePayment(ctx, provider.CreatePaymentInput{
		BookingID:            bookingID,
		Amount:               amount,
		Currency:             currency,
		Provider:             providerName,
		TransactionReference: reference,
	})
	if err != nil {
		_, _ = s.repo.UpdateStatus(ctx, payment.ID, "failed", "")
		return PaymentResult{}, err
	}

	if providerResult.TransactionReference != payment.TransactionReference || payment.Status == "failed" {
		payment, err = s.repo.UpdateStatus(ctx, payment.ID, "pending", providerResult.TransactionReference)
		if err != nil {
			return PaymentResult{}, err
		}
	}

	return PaymentResult{
		Payment:    payment,
		PaymentURL: providerResult.PaymentURL,
	}, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, paymentID int) (PaymentResult, error) {
	if paymentID <= 0 {
		return PaymentResult{}, ErrInvalidPaymentInput
	}

	payment, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return PaymentResult{}, err
	}

	return PaymentResult{
		Payment: payment,
	}, nil
}

func (s *PaymentService) MarkPaymentCompleted(
	ctx context.Context,
	paymentID int,
	transactionReference string,
) (PaymentResult, error) {
	return s.updateStatus(ctx, paymentID, "completed", transactionReference)
}

func (s *PaymentService) MarkPaymentFailed(
	ctx context.Context,
	paymentID int,
	transactionReference string,
) (PaymentResult, error) {
	return s.updateStatus(ctx, paymentID, "failed", transactionReference)
}

func (s *PaymentService) MarkPaymentCompletedByTransactionReference(
	ctx context.Context,
	transactionReference string,
) (PaymentResult, error) {
	return s.updateStatusByTransactionReference(ctx, transactionReference, "completed")
}

func (s *PaymentService) MarkPaymentFailedByTransactionReference(
	ctx context.Context,
	transactionReference string,
) (PaymentResult, error) {
	return s.updateStatusByTransactionReference(ctx, transactionReference, "failed")
}

func (s *PaymentService) updateStatus(
	ctx context.Context,
	paymentID int,
	status string,
	transactionReference string,
) (PaymentResult, error) {
	if paymentID <= 0 {
		return PaymentResult{}, ErrInvalidPaymentInput
	}
	payment, err := s.repo.UpdateStatus(ctx, paymentID, status, transactionReference)
	if err != nil {
		return PaymentResult{}, err
	}

	return PaymentResult{Payment: payment}, nil
}

func (s *PaymentService) updateStatusByTransactionReference(
	ctx context.Context,
	transactionReference string,
	status string,
) (PaymentResult, error) {
	transactionReference = strings.TrimSpace(transactionReference)
	if transactionReference == "" {
		return PaymentResult{}, ErrInvalidPaymentInput
	}
	payment, err := s.repo.UpdateStatusByTransactionReference(ctx, transactionReference, status)
	if err != nil {
		return PaymentResult{}, err
	}

	return PaymentResult{Payment: payment}, nil
}
