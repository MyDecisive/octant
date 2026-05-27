package rpchandler

import (
	"context"
	"errors"
	"maps"
	"slices"

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

	config             *config.Configuration
	datadogIntegration integration.Integration[integration.DataDogIntegrationData]
}

func NewDatadogHandler(
	configuration *config.Configuration,
	datadog integration.Integration[integration.DataDogIntegrationData],
) *DatadogHandler {
	return &DatadogHandler{
		config:             configuration,
		datadogIntegration: datadog,
	}
}

func (dh *DatadogHandler) GetDatadogIntegrations(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[octantv1alpha.GetDatadogIntegrationsResponse], error) {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.DatadogServiceGetDatadogIntegrationsProcedure))

	logger.Debug("received request")

	ddInt, err := dh.datadogIntegration.GetIntegrations(ctx)
	if err != nil {
		logger.Error("Failed to get integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("get integration"))
	}

	return connect.NewResponse(&octantv1alpha.GetDatadogIntegrationsResponse{
		Names: slices.Collect(maps.Keys(ddInt)),
	}), nil
}

func (dh *DatadogHandler) GetDatadogIntegrationByName(
	ctx context.Context,
	req *connect.Request[octantv1alpha.GetDatadogIntegrationByNameRequest],
) (*connect.Response[octantv1alpha.GetDatadogIntegrationByNameResponse], error) {
	integrationName := req.Msg.GetName()
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.DatadogServiceGetDatadogIntegrationByNameProcedure),
		zap.String("integrationName", integrationName),
	)

	logger.Debug("received request")

	integrationData, err := dh.datadogIntegration.GetIntegrationByName(ctx, integrationName)
	if err != nil {
		logger.Error("getting integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse[octantv1alpha.GetDatadogIntegrationByNameResponse](&octantv1alpha.GetDatadogIntegrationByNameResponse{
		Url: integrationData.DDUrl,
	}), nil
}

func (dh *DatadogHandler) SaveDatadogIntegration(
	ctx context.Context,
	request *connect.Request[octantv1alpha.SaveDatadogIntegrationRequest],
) (*connect.Response[emptypb.Empty], error) {
	logger := zap.L().With(zap.String("operation", octantv1alphaconnect.DatadogServiceSaveDatadogIntegrationProcedure))

	logger.Debug("received request")

	if err := dh.datadogIntegration.SetIntegration(
		ctx,
		request.Msg.GetName(),
		integration.DataDogIntegrationData{
			APIKey: request.Msg.GetApiKey(),
			DDUrl:  request.Msg.GetUrl(),
		}); err != nil {
		logger.Error("Failed to save integration", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("save integration"))
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}
