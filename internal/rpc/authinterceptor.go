// Package rpc contains code to handle RPC requests.
package rpc

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewTokenReviewInterceptor returns an interceptor that authenticates every
// request by validating its bearer token against the Kubernetes TokenReview API.
func NewTokenReviewInterceptor(k8sClient kubernetes.Interface) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token, ok := strings.CutPrefix(req.Header().Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
			}

			review, err := k8sClient.AuthenticationV1().TokenReviews().Create(ctx,
				&authenticationv1.TokenReview{
					Spec: authenticationv1.TokenReviewSpec{Token: token},
				}, metav1.CreateOptions{})
			if err != nil || !review.Status.Authenticated {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired token"))
			}

			return next(ctx, req)
		}
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
