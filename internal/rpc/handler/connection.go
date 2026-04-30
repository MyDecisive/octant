package rpchandler

import (
	"context"
	"errors"
	"io"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"go.uber.org/zap"
)

const (
	chunkSize          = 500000 // 500kB
	manifestContentZip = "application/zip"
)

type ConnectionHandler struct {
	octantv1alphaconnect.UnimplementedConnectionServiceHandler

	config           *config.Configuration
	octantConnection connection.Connection[connection.OctantConnectionData]
	compressor       connection.ManifestCompressor
}

func NewConnectionHandler(
	octantConfig *config.Configuration,
	octantConnection connection.Connection[connection.OctantConnectionData],
	compressor connection.ManifestCompressor,
) *ConnectionHandler {
	return &ConnectionHandler{
		config:           octantConfig,
		octantConnection: octantConnection,
		compressor:       compressor,
	}
}

func (ch *ConnectionHandler) GetConnectionStatus(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionStatusRequest],
) (
	*connect.Response[octantv1alpha.GetConnectionStatusResponse],
	error,
) {
	connectionStatus, err := ch.octantConnection.GetConnectionStatus(
		ctx,
		request.Msg.GetNamespace(),
		request.Msg.GetConnectionName(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(connectionStatus), nil
}

func (ch *ConnectionHandler) GenerateManifests(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GenerateManifestsRequest],
	stream *connect.ServerStream[octantv1alpha.GenerateManifestsResponse],
) error {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.DatadogServiceGetDatadogIntegrationsProcedure))

	buf, err := ch.compressor.CreateCompressed(ctx, connection.CompressionInput{
		Namespace:   request.Msg.GetNamespace(),
		Connection:  request.Msg.GetConnectionName(),
		Telemetries: request.Msg.GetTelemetryTypes(),
		Format:      request.Msg.GetFormat(),
	})
	if err != nil {
		logger.Error("Failed to generate manifest zip file", zap.Error(err))
		return connect.NewError(connect.CodeInternal, errors.New("generate zip file"))
	}

	total := buf.Len()
	for {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, end transfer")
			return nil
		default:
		}

		chunk := make([]byte, chunkSize)
		_, err := buf.Read(chunk)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			logger.Error("Failed to read chunks of zip", zap.Error(err))
			return connect.NewError(connect.CodeInternal, errors.New("transferring zip"))
		}
		if err := stream.Send(&octantv1alpha.GenerateManifestsResponse{
			Data:  chunk,
			Total: uint64(total), // nolint:gosec //total will never be negative
			Type:  manifestContentZip,
		}); err != nil {
			logger.Error("Failed to send data chunk", zap.Error(err))
			return connect.NewError(connect.CodeInternal, errors.New("streaming"))
		}
	}
}

func (ch *ConnectionHandler) GetConnectionValidatorRuns(context.Context, *connect.Request[octantv1alpha.GetConnectionValidatorRunsRequest]) (*connect.Response[v1alpha.GetConnectionValidatorRunsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("octant.v1alpha.ConnectionService.GetConnectionValidatorRuns is not implemented"))
}

func (ch *ConnectionHandler) PutConnectionValidatorRun(context.Context, *connect.Request[octantv1alpha.PutConnectionValidatorRunRequest]) (*connect.Response[v1alpha.PutConnectionValidatorRunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("octant.v1alpha.ConnectionService.PutConnectionValidatorRun is not implemented"))
}
