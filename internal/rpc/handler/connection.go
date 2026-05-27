package rpchandler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	budgetfilter "github.com/mydecisive/octant/internal/budget/filter"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
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
	budgetSetting    budgetfilter.SettingController
}

func NewConnectionHandler(
	octantConfig *config.Configuration,
	octantConnection connection.Connection[connection.OctantConnectionData],
	compressor connection.ManifestCompressor,
	setting budgetfilter.SettingController,
) *ConnectionHandler {
	return &ConnectionHandler{
		config:           octantConfig,
		octantConnection: octantConnection,
		compressor:       compressor,
		budgetSetting:    setting,
	}
}

func (ch *ConnectionHandler) GetConnectionStatus(
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionStatusRequest],
) (
	*connect.Response[octantv1alpha.GetConnectionStatusResponse],
	error,
) {
	connScope := request.Msg.GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceGetConnectionStatusProcedure),
		zap.String("namespace", connScope.GetNamespace()),
		zap.String("connectionName", connScope.GetConnectionName()),
	)

	logger.Debug("received request")

	connectionStatus, err := ch.octantConnection.GetConnectionStatus(
		ctx,
		connection.ConnectionCRUDInput{
			Namespace:      connScope.GetNamespace(),
			ConnectionName: connScope.GetConnectionName(),
			Logger:         logger,
		},
		request.Msg.GetValidatorRunId(),
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
	connScope := request.Msg.GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceGenerateManifestsProcedure),
		zap.String("namespace", connScope.GetNamespace()),
		zap.String("connectionName", connScope.GetConnectionName()),
		zap.String("mdaiVersion", request.Msg.GetMdaiVersion()),
	)

	logger.Debug("received request")

	buf, err := ch.compressor.CreateCompressed(ctx, connection.CompressionInput{
		Namespace:   connScope.GetNamespace(),
		Connection:  connScope.GetConnectionName(),
		Telemetries: request.Msg.GetTelemetryTypes(),
		Format:      request.Msg.GetFormat(),
		MdaiVersion: request.Msg.GetMdaiVersion(),
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
		_, err = buf.Read(chunk)
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
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[octantv1alpha.GetConnectionsResponse], error) {
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceGetConnectionsProcedure),
	)

	logger.Debug("received request")

	names, err := ch.octantConnection.GetConnections(ctx, connection.ConnectionCRUDInput{Logger: logger})
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
	connectionName := request.Msg.GetConnectionName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceGetConnectionProcedure),
		zap.String("connectionName", connectionName),
	)

	logger.Debug("received request")

	conn, err := ch.octantConnection.GetConnectionByName(
		ctx,
		connection.ConnectionCRUDInput{
			ConnectionName: connectionName,
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get connection: %w", err))
	}
	if conn == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("connection not found"))
	}

	return connect.NewResponse(convertConnectionDataToGetConnectionResponse(conn)), nil
}

func (ch *ConnectionHandler) CreateConnection(
	ctx context.Context,
	request *connect.Request[octantv1alpha.CreateConnectionRequest],
) (*connect.Response[emptypb.Empty], error) {
	connData := convertRequestToConnectionData(request)
	connScope := request.Msg.GetConnectionData().GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceCreateConnectionProcedure),
		zap.String("connectionName", connScope.GetConnectionName()),
	)

	logger.Debug("received request")

	err := ch.octantConnection.SaveConnection(
		ctx,
		connData,
		connection.ConnectionCRUDInput{
			Namespace:      connScope.GetNamespace(),
			ConnectionName: connScope.GetConnectionName(),
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save connection: %w", err))
	}

	if connData.Deployment != nil && connData.Deployment.Type == connection.ArgoSideloadDeploymentType {
		err := ch.initializeFilterSetting(ctx, connScope, logger)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (ch *ConnectionHandler) GetConnectionValidatorRunIds( // nolint: revive,lll // this fulfills a contract; cannot name like the linter wants
	ctx context.Context,
	request *connect.Request[octantv1alpha.GetConnectionValidatorRunIdsRequest],
) (*connect.Response[octantv1alpha.GetConnectionValidatorRunIdsResponse], error) {
	connScope := request.Msg.GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceGetConnectionValidatorRunIdsProcedure),
		zap.String("connectionName", connScope.GetConnectionName()),
	)

	logger.Debug("received request")

	runs, err := ch.octantConnection.GetConnectionValidatorRuns(
		ctx,
		connection.ConnectionCRUDInput{
			Namespace:      connScope.GetNamespace(),
			ConnectionName: connScope.GetConnectionName(),
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get connection validator runs: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.GetConnectionValidatorRunIdsResponse{
		ValidatorRunIds: runs,
	}), nil
}

func (ch *ConnectionHandler) CreateConnectionValidatorRun(
	ctx context.Context,
	request *connect.Request[octantv1alpha.CreateConnectionValidatorRunRequest],
) (*connect.Response[octantv1alpha.CreateConnectionValidatorRunResponse], error) {
	connScope := request.Msg.GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceCreateConnectionValidatorRunProcedure),
		zap.String("connectionName", connScope.GetConnectionName()),
	)

	logger.Debug("received request")

	runID, err := ch.octantConnection.PutConnectionValidatorRun(
		ctx,
		connection.ConnectionCRUDInput{
			Namespace:      connScope.GetNamespace(),
			ConnectionName: connScope.GetConnectionName(),
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate validator run: %w", err))
	}

	return connect.NewResponse(&octantv1alpha.CreateConnectionValidatorRunResponse{
		ValidatorRunId: runID,
	}), nil
}

func (ch *ConnectionHandler) DeleteConnection(
	ctx context.Context,
	request *connect.Request[octantv1alpha.DeleteConnectionRequest],
) (*connect.Response[emptypb.Empty], error) {
	connectionName := request.Msg.GetConnectionName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceDeleteConnectionProcedure),
		zap.String("connectionName", connectionName),
	)

	logger.Debug("received request")

	err := ch.octantConnection.DeleteConnection(
		ctx,
		connection.ConnectionCRUDInput{
			ConnectionName: connectionName,
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete connection: %w", err))
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (ch *ConnectionHandler) DeleteConnectionValidator(
	ctx context.Context,
	request *connect.Request[octantv1alpha.DeleteConnectionValidatorRequest],
) (*connect.Response[emptypb.Empty], error) {
	connScope := request.Msg.GetScope()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.ConnectionServiceDeleteConnectionValidatorProcedure),
		zap.String("connectionName", connScope.GetConnectionName()),
	)

	logger.Debug("received request")

	err := ch.octantConnection.DeleteConnectionValidator(
		ctx,
		connection.ConnectionCRUDInput{
			Namespace:      connScope.GetNamespace(),
			ConnectionName: connScope.GetConnectionName(),
			Logger:         logger,
		},
	)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete connection: %w", err))
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// initializeFilterSetting initializes the filter settings with the default values from config.
func (ch *ConnectionHandler) initializeFilterSetting(
	ctx context.Context,
	scope *octantv1alpha.ConnectionScope,
	logger *zap.Logger,
) error {
	var filterErr error

	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(ch.config.Budget.FilterSettingUpdateTimeout)*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(1) //nolint: revive // Can't use wg.Go cause I want to pass params to async func
	go func(ctx context.Context, logger *zap.Logger) {
		out := make(chan budgetfilter.UpdateFilterResult)

		go ch.budgetSetting.InitializeFilter(timedCtx, scope.GetNamespace(), scope.GetConnectionName(), out)

		for o := range out {
			select {
			case <-ctx.Done():
				wg.Done()
				logger.Warn("context cancelled while initializing filter settings")
				return
			default:
			}
			if o.Err != nil {
				logger.Error("encountered an error while initializing filter setting",
					zap.String("type", o.Type.String()),
					zap.Error(o.Err),
				)
				if filterErr != nil {
					filterErr = fmt.Errorf("%w:%s %w", filterErr, o.Type.String(), o.Err)
				} else {
					filterErr = fmt.Errorf("%s %w", o.Type.String(), o.Err)
				}
			}

			if o.Status == budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED {
				logger.Debug("completed updating filter status", zap.String("type", o.Type.String()))
			} else {
				logger.Debug("still updating filter status",
					zap.String("type", o.Type.String()),
					zap.String("status", o.Status.String()),
				)
			}
		}
		wg.Done()
	}(timedCtx, logger)
	wg.Wait()

	if filterErr != nil {
		return fmt.Errorf("initialize filter settings: %w", filterErr)
	}
	return nil
}

func convertRequestToConnectionData(
	request *connect.Request[octantv1alpha.CreateConnectionRequest],
) connection.OctantConnectionData {
	destinations := extractDestinationsFromRequest(request)
	dataTypes := extractDataTypesFromRequest(request)
	deployment := extractDeploymentFromRequest(request)
	connData := connection.OctantConnectionData{
		SourceType:     "octant",
		Destinations:   destinations,
		TelemetryTypes: dataTypes,
		Deployment:     deployment,
		MdaiNamespace:  request.Msg.GetConnectionData().GetScope().GetNamespace(),
	}
	return connData
}

func extractDeploymentFromRequest(
	request *connect.Request[octantv1alpha.CreateConnectionRequest],
) *connection.Deployment {
	var deploymentType connection.DeploymentType
	switch request.Msg.GetConnectionData().GetDeployment().GetType() {
	case octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD:
		deploymentType = connection.ArgoSideloadDeploymentType
	case octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST:
		deploymentType = connection.ArgoManifestsDeploymentType
	default:
		deploymentType = ""
	}
	deployment := &connection.Deployment{
		Type:            deploymentType,
		IntegrationName: request.Msg.GetConnectionData().GetDeployment().GetIntegrationName(),
	}
	return deployment
}

func extractDataTypesFromRequest(request *connect.Request[octantv1alpha.CreateConnectionRequest]) []telemetry.MLT {
	var telemetries []telemetry.MLT
	for _, t := range request.Msg.GetConnectionData().GetTelemetryTypes() {
		switch t {
		case octantv1alpha.MLTType_MLT_TYPE_METRIC:
			telemetries = append(telemetries, telemetry.Metrics)
		case octantv1alpha.MLTType_MLT_TYPE_TRACE:
			telemetries = append(telemetries, telemetry.Traces)
		case octantv1alpha.MLTType_MLT_TYPE_LOG:
			telemetries = append(telemetries, telemetry.Logs)
		}
	}
	return telemetries
}

func extractDestinationsFromRequest(
	request *connect.Request[octantv1alpha.CreateConnectionRequest],
) []connection.OctantConnectionDestination {
	var destinations []connection.OctantConnectionDestination
	for _, d := range request.Msg.GetConnectionData().GetDestinations() {
		destType := "unknown"
		if d.GetType() == octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG {
			destType = "datadog"
		}
		destinations = append(destinations, connection.OctantConnectionDestination{
			DestinationType: destType,
			IntegrationName: d.GetIntegrationName(),
		})
	}
	return destinations
}

func convertConnectionDataToGetConnectionResponse(
	conn *connection.OctantConnectionData,
) *octantv1alpha.GetConnectionResponse {
	if conn == nil {
		return nil
	}
	return &octantv1alpha.GetConnectionResponse{
		ConnectionData: &octantv1alpha.ConnectionData{
			TelemetryTypes: convertTelemetryTypesToProtoMLT(conn.TelemetryTypes),
			Deployment:     convertDeploymentToProtoDeployment(conn.Deployment),
			Destinations:   convertDestinationsToProtoDestionations(conn.Destinations),
			Scope: &octantv1alpha.ConnectionScope{
				Namespace:      conn.MdaiNamespace,
				ConnectionName: conn.Deployment.IntegrationName,
			},
		},
	}
}

func convertDeploymentToProtoDeployment(deployment *connection.Deployment) *octantv1alpha.Deployment {
	if deployment == nil {
		return nil
	}

	switch deployment.Type {
	case connection.ArgoSideloadDeploymentType:
		return &octantv1alpha.Deployment{
			Type:            octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_SIDELOAD,
			IntegrationName: deployment.IntegrationName,
		}
	case connection.ArgoManifestsDeploymentType:
		return &octantv1alpha.Deployment{
			Type:            octantv1alpha.DeploymentType_DEPLOYMENT_TYPE_ARGO_MANIFEST,
			IntegrationName: deployment.IntegrationName,
		}
	default:
		return nil
	}
}

func convertTelemetryTypesToProtoMLT(telemetries []telemetry.MLT) []octantv1alpha.MLTType {
	var mltTypes []octantv1alpha.MLTType
	for _, t := range telemetries {
		switch t {
		case telemetry.Metrics:
			mltTypes = append(mltTypes, octantv1alpha.MLTType_MLT_TYPE_METRIC)
		case telemetry.Traces:
			mltTypes = append(mltTypes, octantv1alpha.MLTType_MLT_TYPE_TRACE)
		case telemetry.Logs:
			mltTypes = append(mltTypes, octantv1alpha.MLTType_MLT_TYPE_LOG)
		}
	}
	return mltTypes
}

func convertDestinationsToProtoDestionations(
	destinations []connection.OctantConnectionDestination,
) []*octantv1alpha.TelemetryDestination {
	var contractDestinations []*octantv1alpha.TelemetryDestination
	for _, d := range destinations {
		var destType octantv1alpha.IntegrationType
		switch d.DestinationType {
		case "datadog":
			destType = octantv1alpha.IntegrationType_INTEGRATION_TYPE_DATADOG
		default:
			destType = 0 // Default/unspecified
		}

		contractDestinations = append(contractDestinations, &octantv1alpha.TelemetryDestination{
			Type:            destType,
			IntegrationName: d.IntegrationName,
		})
	}
	return contractDestinations
}
