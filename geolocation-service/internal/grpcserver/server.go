package grpcserver

import (
	"context"

	geolocationpb "diploma/api/geolocation-service-proto"
	"diploma/geolocation-service/internal/repository"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GeolocationService interface {
	FindNearbyWorkers(ctx context.Context, categoryID int, latitude float64, longitude float64, radiusMeters int) ([]repository.NearbyWorker, error)
}

type Server struct {
	geolocationpb.UnimplementedGeolocationServiceServer
	geo GeolocationService
}

func New(geo GeolocationService) *Server {
	return &Server{geo: geo}
}

func (s *Server) FindNearbyWorkers(
	ctx context.Context,
	req *geolocationpb.FindNearbyWorkersRequest,
) (*geolocationpb.FindNearbyWorkersResponse, error) {
	workers, err := s.geo.FindNearbyWorkers(
		ctx,
		int(req.GetCategoryId()),
		req.GetLatitude(),
		req.GetLongitude(),
		int(req.GetRadiusMeters()),
	)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	out := make([]*geolocationpb.NearbyWorker, 0, len(workers))
	for _, worker := range workers {
		out = append(out, &geolocationpb.NearbyWorker{
			WorkerId:        int64(worker.WorkerID),
			FullName:        worker.FullName,
			Price:           int64(worker.Price),
			ExperienceLevel: worker.ExperienceLevel,
			CategoryName:    worker.CategoryName,
			Latitude:        worker.Latitude,
			Longitude:       worker.Longitude,
			DistanceMeters:  worker.DistanceMeters,
		})
	}

	return &geolocationpb.FindNearbyWorkersResponse{Workers: out}, nil
}
