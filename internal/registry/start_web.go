//go:build webapp

package registry

import (
	"fmt"
	"net/http"

	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/rpc"
	"github.com/mydecisive/octant/web"
	"go.uber.org/zap"
)

func Start(con *config.Configuration, rpcServer *rpc.Server) error {
	octantUI, err := web.NewOctantApp()
	if err != nil {
		return fmt.Errorf("creating octant app: %w", err)
	}

	handler, err := rpcServer.Handler()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()

	web.ServeSwaggerUI(mux)

	mux.Handle("/api/", http.StripPrefix("/api", handler))

	mux.Handle("/", http.StripPrefix("/", octantUI.UIHandler()))

	zap.L().Info("starting web and rpc server", zap.Uint16("port", con.Port))
	zap.L().Info("web server will be available from /")
	zap.L().Info("rpc server will be available from /api")
	return http.ListenAndServe(fmt.Sprintf(":%d", con.Port), mux)
}
