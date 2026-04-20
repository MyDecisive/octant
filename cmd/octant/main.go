//go:build !webapp

package main

import (
	"github.com/mydecisive/octant/internal/rpc"
	rpchandler "github.com/mydecisive/octant/internal/rpc/handler"
	"go.uber.org/zap"
)

func main() {
	logger, configuration, cleanup := setup()
	defer cleanup()

	rpcServer := rpc.NewServer(*configuration, rpchandler.NewArgoCDHandler(), rpchandler.NewInstallHandler())

	logger.Fatal("starting server", zap.Error(rpcServer.Start()))
}
