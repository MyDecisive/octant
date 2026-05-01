package rpchandler

import (
	"context"
	"errors"
	"fmt"
	"github.com/mydecisive/octant/internal/telemetry"
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
func (ch *ConnectionHandler) GetConnections(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionsRequest],
) (*connect.Response[octantv1alpha.GetConnectionsResponse], error) {
	names, err := ch.octantConnection.GetConnections(ctx, request.Msg.GetNamespace())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get connections: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.GetConnectionsResponse{
		ConnectionNames: names,
	}), nil
}

func (ch *ConnectionHandler) GetConnection(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionRequest],
) (*connect.Response[octantv1alpha.GetConnectionResponse], error) {
	conn, err := ch.octantConnection.GetConnectionByName(
		ctx,
		request.Msg.GetNamespace(),
		request.Msg.GetConnectionName(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get connection: %w", err))
	}
	if conn == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}

	var telemetryTypes []octantv1alpha.MLTType
	for _, t := range conn.TelemetryTypes {
		if val, ok := octantv1alpha.MLTType_value[string(t)]; ok {
			telemetryTypes = append(telemetryTypes, octantv1alpha.MLTType(val))
		}
	}

	var destinations []*octantv1alpha.TelemetryDestination
	for _, d := range conn.Destinations {
		var destType octantv1alpha.IntegrationType
		if d.DestinationType == "datadog" {
			destType = octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG
		}
		destinations = append(destinations, &octantv1alpha.TelemetryDestination{
			Type:            destType,
			IntegrationName: d.IntegrationName,
		})
	}

	var deploymentType octantv1alpha.DeploymentType
	if conn.Deployment != nil {
		if val, ok := octantv1alpha.DeploymentType_value[string(conn.Deployment.Type)]; ok {
			deploymentType = octantv1alpha.DeploymentType(val)
		}
	}

	return connect.NewResponse(&octantv1alpha.GetConnectionResponse{
		TelemetryTypes: telemetryTypes,
		DeploymentType: deploymentType,
		Destinations:   destinations,
	}), nil
}

func (ch *ConnectionHandler) PutConnection(
	ctx context.Context,
	request *connect.Request[octantv1alpha.PutConnectionRequest],
) (*connect.Response[octantv1alpha.PutConnectionResponse], error) {
	var destinations []connection.OctantConnectionDestination
	for _, d := range request.Msg.GetDestinations() {
		destType := "unknown"
		if d.GetType() == octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG {
			destType = "datadog"
		}
		destinations = append(destinations, connection.OctantConnectionDestination{
			DestinationType: destType,
			IntegrationName: d.GetIntegrationName(),
		})
	}

	var telemetries []telemetry.MLT
	for _, t := range request.Msg.GetTelemetryTypes() {
		telemetries = append(telemetries, telemetry.MLT(t.String()))
	}

	var deploymentType connection.DeploymentType
	switch request.Msg.GetDeployment().GetType() {
	case octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD:
		deploymentType = connection.ArgoSideloadDeploymentType
	case octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST:
		deploymentType = connection.ArgoManifestsDeploymentType
	default:
		deploymentType = ""
	}
	connData := connection.OctantConnectionData{
		SourceType:     "octant",
		Destinations:   destinations,
		TelemetryTypes: telemetries,
		Deployment: &connection.Deployment{
			Type:            deploymentType,
			IntegrationName: request.Msg.GetDeployment().GetIntegrationName(),
		},
	}

	runID, err := ch.octantConnection.SaveConnection(
		ctx,
		connData,
		request.Msg.GetNamespace(),
		request.Msg.GetConnectionName(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save connection: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.PutConnectionResponse{
		ValidatorRunId: runID,
	}), nil
}

func (ch *ConnectionHandler) GetConnectionValidatorRuns(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionValidatorRunsRequest],
) (*connect.Response[octantv1alpha.GetConnectionValidatorRunsResponse], error) {
	runs, err := ch.octantConnection.GetConnectionValidatorRuns(
		ctx,
		request.Msg.GetNamespace(),
		request.Msg.GetConnectionName(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get connection validator runs: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.GetConnectionValidatorRunsResponse{
		ValidatorRunIds: runs,
	}), nil
}

func (ch *ConnectionHandler) PutConnectionValidatorRun(
	ctx context.Context,
	request *connect.Request[octantv1alpha.PutConnectionValidatorRunRequest],
) (*connect.Response[octantv1alpha.PutConnectionValidatorRunResponse], error) {
	runID, err := ch.octantConnection.PutConnectionValidatorRun(
		ctx,
		request.Msg.GetNamespace(),
		request.Msg.GetConnectionName(),
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate validator run: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.PutConnectionValidatorRunResponse{
		ValidatorRunId: runID,
	}), nil
}
