package rpchandler

import (
	"context"
	"errors"
	"io"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/connection"
	"go.uber.org/zap"
)

const (
	chunkSize          = 500000 // 500kB
	manifestContentZip = "application/zip"
)

type ConnectionHandler struct {
	octantv1alphaconnect.UnimplementedConnectionServiceHandler

	compressor connection.ManifestCompressor
}

func NewConnectionHandler(compressor connection.ManifestCompressor) *ConnectionHandler {
	return &ConnectionHandler{
		compressor: compressor,
	}
}

func (ch *ConnectionHandler) GenerateManifests(ctx context.Context, request *connect.Request[octantv1alpha.GenerateManifestsRequest], stream *connect.ServerStream[octantv1alpha.GenerateManifestsResponse]) error {
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
			Total: uint64(total),
			Type:  manifestContentZip,
		}); err != nil {
			logger.Error("Failed to send data chunk", zap.Error(err))
			return connect.NewError(connect.CodeInternal, errors.New("streaming"))
		}
	}
}
