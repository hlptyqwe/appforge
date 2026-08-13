package i18n

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func StatusError(ctx context.Context, code int32) error {
	return status.Error(codes.Code(uint32(code)), Translate(code, ctx))
}

func IsStatusError(err error, code int32) bool {
	if err == nil {
		return false
	}
	for {
		if status.Code(err) == codes.Code(uint32(code)) {
			return true
		}
		next := errors.Unwrap(err)
		if next == nil {
			return false
		}
		err = next
	}
}
