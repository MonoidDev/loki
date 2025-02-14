package server

import (
	"context"
	"errors"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/grafana/loki/pkg/util"

	"github.com/prometheus/prometheus/promql"
	"github.com/weaveworks/common/httpgrpc"
	"github.com/weaveworks/common/user"

	"github.com/grafana/loki/pkg/logqlmodel"
	"github.com/grafana/loki/pkg/storage/chunk"
)

// StatusClientClosedRequest is the status code for when a client request cancellation of an http request
const StatusClientClosedRequest = 499

const (
	ErrClientCanceled   = "The request was cancelled by the client."
	ErrDeadlineExceeded = "Request timed out, decrease the duration of the request or add more label matchers (prefer exact match over regex match) to reduce the amount of data processed."
)

// WriteError write a go error with the correct status code.
func WriteError(err error, w http.ResponseWriter) {
	var (
		queryErr chunk.QueryError
		promErr  promql.ErrStorage
	)

	me, ok := err.(util.MultiError)
	if ok && me.IsCancel() {
		http.Error(w, ErrClientCanceled, StatusClientClosedRequest)
		return
	}
	if ok && me.IsDeadlineExceeded() {
		http.Error(w, ErrDeadlineExceeded, http.StatusGatewayTimeout)
		return
	}

	s, isRPC := status.FromError(err)
	switch {
	case errors.Is(err, context.Canceled) ||
		(isRPC && s.Code() == codes.Canceled) ||
		(errors.As(err, &promErr) && errors.Is(promErr.Err, context.Canceled)):
		http.Error(w, ErrClientCanceled, StatusClientClosedRequest)
	case errors.Is(err, context.DeadlineExceeded) ||
		(isRPC && s.Code() == codes.DeadlineExceeded):
		http.Error(w, ErrDeadlineExceeded, http.StatusGatewayTimeout)
	case errors.As(err, &queryErr):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, logqlmodel.ErrLimit) || errors.Is(err, logqlmodel.ErrParse) || errors.Is(err, logqlmodel.ErrPipeline):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, user.ErrNoOrgID):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		if grpcErr, ok := httpgrpc.HTTPResponseFromError(err); ok {
			http.Error(w, string(grpcErr.Body), int(grpcErr.Code))
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
