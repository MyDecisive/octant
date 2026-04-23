// Package rpchandler contains handlers that will handle RPC service calls.
package rpchandler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/integration"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
)

type DatadogHandler struct {
	octantv1alphaconnect.UnimplementedDatadogServiceHandler

	config  *config.Configuration
	datadog integration.Integration[integration.DataDogIntegrationData]
}

func NewDatadogHandler(config *config.Configuration, datadog integration.Integration[integration.DataDogIntegrationData]) *DatadogHandler {
	return &DatadogHandler{
		config:  config,
		datadog: datadog,
	}
}

func (dh *DatadogHandler) GetDatadogIntegrations(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[octantv1alpha.GetDatadogIntegrationsResponse], error) {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.DatadogServiceGetDatadogIntegrationsProcedure))
	ddInt, err := dh.datadog.GetIntegrations(ctx, dh.config.CurrentNamespace)
	if err != nil {
		logger.Error("Failed to get integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("get integration"))
	}

	names := make([]string, 0, len(ddInt))
	for intName := range ddInt {
		names = append(names, intName)
	}

	return connect.NewResponse(&octantv1alpha.GetDatadogIntegrationsResponse{
		Names: names,
	}), nil
}

func (dh *DatadogHandler) SaveDatadogIntegration(ctx context.Context, request *connect.Request[octantv1alpha.SaveDatadogIntegrationRequest]) (*connect.Response[emptypb.Empty], error) {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.DatadogServiceSaveDatadogIntegrationProcedure))

	if err := dh.datadog.SetIntegration(ctx, dh.config.CurrentNamespace, request.Msg.Name, integration.DataDogIntegrationData{
		APIKey: request.Msg.ApiKey,
		DDUrl:  request.Msg.Url,
	}); err != nil {
		logger.Error("Failed to save integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("save integration"))
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
