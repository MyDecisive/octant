package argocd

import (
	"context"
	"net"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc"
)

type mockAppServerV3 struct {
	application.UnimplementedApplicationServiceServer
}

func (*mockAppServerV3) List(
	ctx context.Context,
	req *application.ApplicationQuery,
) (*v1alpha1.ApplicationList, error) {
	return &v1alpha1.ApplicationList{}, nil
}

func TestTestConnection(t *testing.T) {
	t.Parallel()

	lis, err := net.Listen("tcp", "127.0.0.1:0") // nolint: noctx
	require.NoError(t, err)

	s := grpc.NewServer()
	application.RegisterApplicationServiceServer(s, &mockAppServerV3{})

	// start the mock app service server
	go s.Serve(lis) // nolint: errcheck

	t.Cleanup(s.Stop)

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		testClient := NewArgoCDClient()
		success, err := testClient.TestConnection(t.Context(), zaptest.NewLogger(t), &apiclient.ClientOptions{
			ServerAddr: lis.Addr().String(),
			Insecure:   true,
			PlainText:  true, // needed for local testing
		})
		require.NoError(t, err)
		require.True(t, success)
	})
}
