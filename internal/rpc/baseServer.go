// Package rpc contains code to handle RPC requests.
package rpc

import (
	"fmt"
	"github.com/mydecisive/octant/internal/config"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/validate"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type BaseServer struct{}

// Run will perform any necessary setups and then start the server.
func (BaseServer) Run(
	env config.Environment,
	port uint16,
	services []string,
	handlers []func() (string, http.Handler),
) error {
	mux := http.NewServeMux()

	if env == config.Dev {
		// Allow auto-discovery of schemas in dev environment
		reflector := grpcreflect.NewStaticReflector(services...)
		mux.Handle(grpcreflect.NewHandlerV1(reflector))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	}
	// Healthcheck endpoints
	checker := grpchealth.NewStaticChecker(services...)
	mux.Handle(grpchealth.NewHandler(checker))

	// Service Handlers
	for _, handler := range handlers {
		mux.Handle(handler())
	}

	// Serve HTTP/2 without TLS.
	return http.ListenAndServe( //nolint:gosec // setting timeout handled by RPC server.
		fmt.Sprintf(":%d", port),
		h2c.NewHandler(mux, &http2.Server{}),
	)
}

// GetInterceptors returns list of interceptors to be applied to all services as an option.
func (BaseServer) GetInterceptors() (connect.Option, error) {
	interceptor := validate.NewInterceptor()

	return connect.WithInterceptors(interceptor), nil
}
