package grpcserver

import (
	"context"

	usermanagementpb "diploma/api/usermanagement-service-proto"
	"diploma/usermanagement-service/internal/model"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService interface {
	GetCustomerProfile(ctx context.Context, userID int) (*model.CustomerProfile, error)
	GetWorkerProfile(ctx context.Context, userID int) (*model.WorkerProfile, error)
	HasPaymentMethod(ctx context.Context, userID int) (bool, error)
}

type Server struct {
	usermanagementpb.UnimplementedUserManagementServiceServer
	auth AuthService
}

func New(auth AuthService) *Server {
	return &Server{auth: auth}
}

func (s *Server) GetCustomerProfile(
	ctx context.Context,
	req *usermanagementpb.GetProfileRequest,
) (*usermanagementpb.CustomerProfileResponse, error) {
	userID := req.GetUserId()
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	profile, err := s.auth.GetCustomerProfile(ctx, int(userID))
	if err != nil {
		return nil, status.Error(codes.NotFound, "customer profile not found")
	}

	return &usermanagementpb.CustomerProfileResponse{
		CustomerProfileId: int64(profile.ID),
	}, nil
}

func (s *Server) GetWorkerProfile(
	ctx context.Context,
	req *usermanagementpb.GetProfileRequest,
) (*usermanagementpb.WorkerProfileResponse, error) {
	userID := req.GetUserId()
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	profile, err := s.auth.GetWorkerProfile(ctx, int(userID))
	if err != nil {
		return nil, status.Error(codes.NotFound, "worker profile not found")
	}

	return &usermanagementpb.WorkerProfileResponse{
		WorkerProfileId: int64(profile.ID),
	}, nil
}

func (s *Server) HasPaymentMethod(
	ctx context.Context,
	req *usermanagementpb.GetProfileRequest,
) (*usermanagementpb.HasPaymentMethodResponse, error) {
	userID := req.GetUserId()
	if userID <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id must be positive")
	}

	hasPaymentMethod, err := s.auth.HasPaymentMethod(ctx, int(userID))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &usermanagementpb.HasPaymentMethodResponse{
		HasPaymentMethod: hasPaymentMethod,
	}, nil
}
