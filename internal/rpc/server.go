// Package rpc contains code to handle RPC requests.
package rpc

import (
	"github.com/mydecisive/octant/internal/config"
	"net/http"

	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"

	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
)

// Server that will serve internal RPC endpoint handlers.
type Server struct {
	BaseServer

	configuration  config.Configuration
	argocdHandler  *rpchandler.ArgoCDHandler
	installHandler *rpchandler.InstallHandler
}

// NewServer create a new Server.
func NewServer(
	configuration config.Configuration,
	argocdHandler *rpchandler.ArgoCDHandler,
	installHandler *rpchandler.InstallHandler,
) *Server {
	return &Server{
		configuration:  configuration,
		argocdHandler:  argocdHandler,
		installHandler: installHandler,
	}
}

// Start will perform any necessary setups and then start the server.
func (s Server) Start() error {
	interceptors, err := s.GetInterceptors()
	if err != nil {
		return err
	}
	return s.Run(s.configuration.Env, s.configuration.RPC.Port, s.getServices(), []func() (string, http.Handler){
		func() (string, http.Handler) {
			return octantv1alphaconnect.NewArgoCDServiceHandler(s.argocdHandler, interceptors)
		},
		func() (string, http.Handler) {
			return octantv1alphaconnect.NewInstallServiceHandler(s.installHandler, interceptors)
		},
	})
}

// getServices returns the list of pre-defined services this RPC server serves.
//
// When adding a new service, don't forget to update this list.
func (Server) getServices() []string {
	return []string{
		octantv1alphaconnect.ArgoCDServiceName,
		octantv1alphaconnect.InstallServiceName,
	}
}
