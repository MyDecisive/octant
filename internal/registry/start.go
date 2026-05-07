//go:build !webapp

package registry

import (
	"fmt"
	"net/http"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/rpc"
	"go.uber.org/zap"
)

func Start(con *config.Configuration, rpcServer *rpc.Server) error {
	zap.L().Info("starting rpc server", zap.Uint16("port", con.Port))
	handler, err := rpcServer.Handler()
	if err != nil {
		return err
	}

	zap.L().Info("rpc server will be available from /")
	return http.ListenAndServe( //nolint:gosec // setting timeout handled by RPC server.
		fmt.Sprintf(":%d", con.Port),
		handler,
	)
}
