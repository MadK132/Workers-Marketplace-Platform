package grpcserver

import (
	"context"
	"errors"
	"time"

	bookingpb "diploma/api/booking-service-proto"
	"diploma/booking-service/internal/repository"
	"diploma/booking-service/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type RequestService interface {
	CreateRequest(ctx context.Context, customerProfileID int, categoryID int, description string, address string, latitude float64, longitude float64) error
	ListCustomerRequests(ctx context.Context, customerProfileID int) ([]repository.RequestListItem, error)
}

type BookingService interface {
	CreateBooking(ctx context.Context, requestID int, workerProfileID int, customerProfileID int) error
	ListCustomerBookings(ctx context.Context, customerProfileID int) ([]repository.BookingListItem, error)
	ListWorkerBookings(ctx context.Context, workerProfileID int) ([]repository.BookingListItem, error)
	StartBooking(ctx context.Context, bookingID int, workerProfileID int) error
	CompleteBooking(ctx context.Context, bookingID int, workerProfileID int) error
}

type Server struct {
	bookingpb.UnimplementedBookingServiceServer
	requests RequestService
	bookings BookingService
}

func New(requests RequestService, bookings BookingService) *Server {
	return &Server{
		requests: requests,
		bookings: bookings,
	}
}

func (s *Server) CreateRequest(
	ctx context.Context,
	req *bookingpb.CreateRequestRequest,
) (*bookingpb.CreateRequestResponse, error) {
	if req.GetCustomerProfileId() <= 0 || req.GetCategoryId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "customer_profile_id and category_id must be positive")
	}
	if req.GetDescription() == "" {
		return nil, status.Error(codes.InvalidArgument, "description is required")
	}

	err := s.requests.CreateRequest(
		ctx,
		int(req.GetCustomerProfileId()),
		int(req.GetCategoryId()),
		req.GetDescription(),
		"Astana",
		51.1694,
		71.4491,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &bookingpb.CreateRequestResponse{Message: "request created"}, nil
}

func (s *Server) ListCustomerRequests(
	ctx context.Context,
	req *bookingpb.ListCustomerRequestsRequest,
) (*bookingpb.ListCustomerRequestsResponse, error) {
	if req.GetCustomerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "customer_profile_id must be positive")
	}

	items, err := s.requests.ListCustomerRequests(ctx, int(req.GetCustomerProfileId()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	out := make([]*bookingpb.ServiceRequest, 0, len(items))
	for _, item := range items {
		out = append(out, mapRequest(item))
	}

	return &bookingpb.ListCustomerRequestsResponse{Requests: out}, nil
}

func (s *Server) CreateBooking(
	ctx context.Context,
	req *bookingpb.CreateBookingRequest,
) (*bookingpb.CreateBookingResponse, error) {
	if req.GetRequestId() <= 0 || req.GetWorkerProfileId() <= 0 || req.GetCustomerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "request_id, worker_profile_id and customer_profile_id must be positive")
	}

	err := s.bookings.CreateBooking(
		ctx,
		int(req.GetRequestId()),
		int(req.GetWorkerProfileId()),
		int(req.GetCustomerProfileId()),
	)
	if err != nil {
		return nil, bookingError(err)
	}

	return &bookingpb.CreateBookingResponse{Message: "booking created"}, nil
}

func (s *Server) ListCustomerBookings(
	ctx context.Context,
	req *bookingpb.ListCustomerBookingsRequest,
) (*bookingpb.ListBookingsResponse, error) {
	if req.GetCustomerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "customer_profile_id must be positive")
	}

	items, err := s.bookings.ListCustomerBookings(ctx, int(req.GetCustomerProfileId()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return mapBookings(items), nil
}

func (s *Server) ListWorkerBookings(
	ctx context.Context,
	req *bookingpb.ListWorkerBookingsRequest,
) (*bookingpb.ListBookingsResponse, error) {
	if req.GetWorkerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "worker_profile_id must be positive")
	}

	items, err := s.bookings.ListWorkerBookings(ctx, int(req.GetWorkerProfileId()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return mapBookings(items), nil
}

func (s *Server) StartBooking(
	ctx context.Context,
	req *bookingpb.UpdateBookingStatusRequest,
) (*bookingpb.UpdateBookingStatusResponse, error) {
	if req.GetBookingId() <= 0 || req.GetWorkerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "booking_id and worker_profile_id must be positive")
	}

	err := s.bookings.StartBooking(ctx, int(req.GetBookingId()), int(req.GetWorkerProfileId()))
	if err != nil {
		return nil, bookingError(err)
	}

	return &bookingpb.UpdateBookingStatusResponse{Message: "booking started"}, nil
}

func (s *Server) CompleteBooking(
	ctx context.Context,
	req *bookingpb.UpdateBookingStatusRequest,
) (*bookingpb.UpdateBookingStatusResponse, error) {
	if req.GetBookingId() <= 0 || req.GetWorkerProfileId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "booking_id and worker_profile_id must be positive")
	}

	err := s.bookings.CompleteBooking(ctx, int(req.GetBookingId()), int(req.GetWorkerProfileId()))
	if err != nil {
		return nil, bookingError(err)
	}

	return &bookingpb.UpdateBookingStatusResponse{Message: "booking completed"}, nil
}

func bookingError(err error) error {
	switch {
	case errors.Is(err, service.ErrRequestNotFound), errors.Is(err, service.ErrBookingNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrRequestNotOwned), errors.Is(err, service.ErrBookingNotOwned):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrRequestUnavailable),
		errors.Is(err, service.ErrWorkerNotSelectable),
		errors.Is(err, service.ErrInvalidTransition):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func mapRequest(item repository.RequestListItem) *bookingpb.ServiceRequest {
	return &bookingpb.ServiceRequest{
		RequestId:    int64(item.RequestID),
		CategoryId:   int64(item.CategoryID),
		CategoryName: item.CategoryName,
		Description:  item.Description,
		Status:       requestStatus(item.Status),
		CreatedAt:    timestamppb.New(item.CreatedAt),
	}
}

func mapBookings(items []repository.BookingListItem) *bookingpb.ListBookingsResponse {
	out := make([]*bookingpb.Booking, 0, len(items))
	for _, item := range items {
		out = append(out, mapBooking(item))
	}
	return &bookingpb.ListBookingsResponse{Bookings: out}
}

func mapBooking(item repository.BookingListItem) *bookingpb.Booking {
	return &bookingpb.Booking{
		BookingId:          int64(item.BookingID),
		RequestId:          int64(item.RequestID),
		WorkerProfileId:    int64(item.WorkerProfileID),
		CustomerProfileId:  int64(item.CustomerProfileID),
		CategoryId:         int64(item.CategoryID),
		CategoryName:       item.CategoryName,
		RequestDescription: item.RequestDescription,
		Status:             bookingStatus(item.Status),
		ScheduledTime:      timestamp(item.ScheduledTime),
		StartTime:          timestamp(item.StartTime),
		EndTime:            timestamp(item.EndTime),
		FinalPrice:         item.FinalPrice,
		CounterpartyName:   item.CounterpartyName,
		CounterpartyRole:   item.CounterpartyRole,
	}
}

func requestStatus(value string) bookingpb.RequestStatus {
	switch value {
	case "pending":
		return bookingpb.RequestStatus_REQUEST_STATUS_PENDING
	case "accepted":
		return bookingpb.RequestStatus_REQUEST_STATUS_ACCEPTED
	case "in_progress":
		return bookingpb.RequestStatus_REQUEST_STATUS_IN_PROGRESS
	case "completed":
		return bookingpb.RequestStatus_REQUEST_STATUS_COMPLETED
	default:
		return bookingpb.RequestStatus_REQUEST_STATUS_UNSPECIFIED
	}
}

func bookingStatus(value string) bookingpb.BookingStatus {
	switch value {
	case "scheduled":
		return bookingpb.BookingStatus_BOOKING_STATUS_SCHEDULED
	case "in_progress":
		return bookingpb.BookingStatus_BOOKING_STATUS_IN_PROGRESS
	case "completed":
		return bookingpb.BookingStatus_BOOKING_STATUS_COMPLETED
	default:
		return bookingpb.BookingStatus_BOOKING_STATUS_UNSPECIFIED
	}
}

func timestamp(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
