package driver

import (
	"errors"

	"github.com/brimble/nobus-csi/internal/cloud"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func statusError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, cloud.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, cloud.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, cloud.ErrQuotaExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	case errors.Is(err, cloud.ErrConflict):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, cloud.ErrRateLimited), errors.Is(err, cloud.ErrUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, cloud.ErrOutOfRange):
		return status.Error(codes.OutOfRange, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
