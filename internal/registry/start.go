//go:build !webapp

package registry

import "github.com/mydecisive/octant/internal/rpc"

func Start(rpcServer *rpc.Server) error {
	return rpcServer.Start()
}
