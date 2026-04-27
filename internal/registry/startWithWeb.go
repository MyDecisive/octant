//go:build webapp

package registry

import (
	"context"
	"fmt"

	"github.com/mydecisive/octant/internal/rpc"
	"github.com/mydecisive/octant/web"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func Start(rpcServer *rpc.Server) error {
	ui, err := web.CreateServer()
	if err != nil {
		return fmt.Errorf("ui server: %w", err)
	}

	// Init Servers
	g, _ := errgroup.WithContext(context.Background())

	// Start servers
	g.Go(func() error {
		return fmt.Errorf("rpc server: %w", rpcServer.Start())
	})
	g.Go(func() error {
		return fmt.Errorf("UI server: %w", ui.ListenAndServe())
	})
	zap.L().Info("Start all servers")
	return g.Wait()
}
