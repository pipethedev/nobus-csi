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
		return status.Error(codes.NotFound, "resource not found")
	case errors.Is(err, cloud.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "resource already exists")
	case errors.Is(err, cloud.ErrQuotaExceeded):
		return status.Error(codes.ResourceExhausted, "provider quota exhausted")
	case errors.Is(err, cloud.ErrConflict):
		return status.Error(codes.FailedPrecondition, "resource is in an incompatible state")
	case errors.Is(err, cloud.ErrRateLimited), errors.Is(err, cloud.ErrUnavailable):
		return status.Error(codes.Unavailable, "provider is unavailable")
	case errors.Is(err, cloud.ErrOutOfRange):
		return status.Error(codes.OutOfRange, "requested capacity is out of range")
	case errors.Is(err, cloud.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, "provider credentials are not authorized")
	case errors.Is(err, cloud.ErrInvalid):
		return status.Error(codes.InvalidArgument, "provider rejected the request")
	default:
		return status.Error(codes.Internal, "internal driver error")
	}
}

func grpcError(err error) error {
	if _, ok := status.FromError(err); ok {
		return err
	}
	return statusError(err)
}
