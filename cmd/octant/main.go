//go:build !webapp

package main

import (
	"github.com/mydecisive/octant/internal/argocd"
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"go.uber.org/zap"
)

func main() {
	logger, configuration, cleanup := setup()
	defer cleanup()

	rpcServer := rpc.NewServer(
		*configuration,
		rpchandler.NewArgoCDHandler(configuration, argocd.NewArgoCDClient()),
		rpchandler.NewInstallHandler(),
	)

	logger.Info("starting RPC server", zap.Int("port", int(configuration.RPC.Port)))
	logger.Fatal("starting server", zap.Error(rpcServer.Start()))
}
