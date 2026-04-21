package rpchandler

import (
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"io"
	"log"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestInstallHandler_InstallMDAIHub(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		handler := NewInstallHandler()
		response, err := handler.InstallMDAIHub(
			t.Context(),
			connect.NewRequest(&octantv1alpha.InstallMDAIHubRequest{
				Namespace: "mdai",
			}),
		)
		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, &connect.Response[emptypb.Empty]{}, response)
	})
}

func TestInstallHandler_GetInstallStatus(t *testing.T) {
	t.Parallel()

	// setup the install handler and test server
	handler := NewInstallHandler()
	installServiceMethods := octantv1alpha.File_octant_v1alpha_install_service_proto.Services().ByName("InstallService").Methods()
	installServiceGetInstallStatusHandler := connect.NewServerStreamHandler(
		octantv1alphaconnect.InstallServiceGetInstallStatusProcedure,
		handler.GetInstallStatus,
		connect.WithSchema(installServiceMethods.ByName("GetInstallStatus")),
	)

	testServer := httptest.NewUnstartedServer(installServiceGetInstallStatusHandler)
	testServer.Config.ErrorLog = log.New(io.Discard, "", 0) //nolint:forbidigo
	testServer.EnableHTTP2 = true
	testServer.StartTLS()
	t.Cleanup(testServer.Close)

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		client := octantv1alphaconnect.NewInstallServiceClient(testServer.Client(), testServer.URL, connect.WithSendGzip())
		response, err := client.GetInstallStatus(t.Context(), connect.NewRequest(&octantv1alpha.GetInstallStatusRequest{
			HubName: "coolHub",
		}))
		require.NoError(t, err)
		require.NotNil(t, response)

		for response.Receive() {
		} // wait to receive all response stream messages

		require.NoError(t, response.Err())
		require.NoError(t, response.Close())
	})
}
