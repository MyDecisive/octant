//go:build !webapp

package registry

import (
	"github.com/mydecisive/octant/internal/rpc"
	"go.uber.org/zap"
)

func Start(rpcServer *rpc.Server) error {
	zap.L().Info("starting rpc server")
	return rpcServer.Start()
}
