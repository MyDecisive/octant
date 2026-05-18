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
	uiHandler, err := web.OctantUIHandler()
	if err != nil {
		return fmt.Errorf("ui server: %w", err)
	}

	swaggerUIHandler, err := web.SwaggerUIHandler()
	if err != nil {
		return fmt.Errorf("creating swagger ui handler: %w", err)
	}

	handler, err := rpcServer.Handler()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", uiHandler)
	mux.Handle("/docs/", http.StripPrefix("/docs/", swaggerUIHandler))
	mux.HandleFunc("/docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, readErr := web.SwaggerUI.ReadFile("octant_docs.swagger.json")
		if readErr != nil {
			zap.L().Error("Error reading docs file", zap.Error(readErr))
		}
		w.Write(data)
	})
	mux.Handle("/api/", http.StripPrefix("/api", handler))

	zap.L().Info("starting web and rpc server", zap.Uint16("port", con.Port))
	zap.L().Info("web server will be available from /")
	zap.L().Info("rpc server will be available from /api")
	return http.ListenAndServe(fmt.Sprintf(":%d", con.Port), mux)
}
