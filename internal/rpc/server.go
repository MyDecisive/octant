// Package rpc contains code to handle RPC requests.
package rpc

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"
	"connectrpc.com/validate"
	"github.com/mydecisive/octant/internal/config"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"

	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
)

// Server that will serve internal RPC endpoint handlers.
type Server struct {
	configuration *config.Configuration

	argocdHandler     *rpchandler.ArgoCDHandler
	installHandler    *rpchandler.InstallHandler
	datadogHandler    *rpchandler.DatadogHandler
	connectionHandler *rpchandler.ConnectionHandler
}

// NewServer create a new Server.
func NewServer(
	configuration *config.Configuration,
	argocdHandler *rpchandler.ArgoCDHandler,
	installHandler *rpchandler.InstallHandler,
	datadogHandler *rpchandler.DatadogHandler,
	connectionHandler *rpchandler.ConnectionHandler,
) *Server {
	return &Server{
		configuration:     configuration,
		argocdHandler:     argocdHandler,
		installHandler:    installHandler,
		datadogHandler:    datadogHandler,
		connectionHandler: connectionHandler,
	}
}

// Start will perform any necessary setups and then start the server.
func (s Server) Start() error {
	interceptors, err := s.getInterceptors()
	if err != nil {
		return err
	}
	services := s.getServices()
	mux := http.NewServeMux()

	if s.configuration.Env == config.Dev {
		// Allow auto-discovery of schemas in dev environment
		reflector := grpcreflect.NewStaticReflector(services...)
		mux.Handle(grpcreflect.NewHandlerV1(reflector))
		mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	}
	// Healthcheck endpoints
	checker := grpchealth.NewStaticChecker(services...)
	mux.Handle(grpchealth.NewHandler(checker))

	// Service Handlers
	mux.Handle(octantv1alphaconnect.NewArgoCDServiceHandler(s.argocdHandler, interceptors))
	mux.Handle(octantv1alphaconnect.NewInstallServiceHandler(s.installHandler, interceptors))
	mux.Handle(octantv1alphaconnect.NewDatadogServiceHandler(s.datadogHandler, interceptors))
	mux.Handle(octantv1alphaconnect.NewConnectionServiceHandler(s.connectionHandler, interceptors))

	// Serve HTTP/2 without TLS.
	return http.ListenAndServe( //nolint:gosec // setting timeout handled by RPC server.
		fmt.Sprintf(":%d", s.configuration.RPC.Port),
		h2c.NewHandler(mux, &http2.Server{}),
	)
}

// getServices returns the list of pre-defined services this RPC server serves.
//
// When adding a new service, don't forget to update this list.
func (Server) getServices() []string {
	return []string{
		octantv1alphaconnect.ArgoCDServiceName,
		octantv1alphaconnect.InstallServiceName,
		octantv1alphaconnect.DatadogServiceName,
		octantv1alphaconnect.ConnectionServiceName,
	}
}

// getInterceptors returns list of interceptors to be applied to all services as an option.
func (Server) getInterceptors() (connect.Option, error) {
	validateInterceptor := validate.NewInterceptor()
	otelInterceptor, err := otelconnect.NewInterceptor(
		otelconnect.WithoutServerPeerAttributes(), // https://connectrpc.com/docs/go/observability#reducing-metrics-and-tracing-cardinality
	)
	if err != nil {
		return nil, fmt.Errorf("create otelconnect interceptor: %w", err)
	}

	return connect.WithInterceptors(validateInterceptor, otelInterceptor), nil
}
