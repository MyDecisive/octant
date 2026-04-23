//go:build !webapp

package main

import (
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/integration"
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"go.uber.org/zap"
)

func main() {
	deps, cleanup := setup()
	defer cleanup()

	rpcServer := rpc.NewServer(
		*deps.config,
		rpchandler.NewArgoCDHandler(deps.config, argocd.NewArgoCDClient(), &integration.ArgoCDIntegration{
			K8sClient: deps.k8sClient,
		}),
		rpchandler.NewInstallHandler(deps.config, argocd.NewArgoCDClient(), &integration.ArgoCDIntegration{
			K8sClient: deps.k8sClient,
		}),
	)

	deps.logger.Info("starting RPC server", zap.Int("port", int(deps.config.RPC.Port)))
	deps.logger.Fatal("starting server", zap.Error(rpcServer.Start()))
}
