package rpchandler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha/octantv1alphaconnect"
	"github.com/mydecisive/octant/internal/setting"
	"go.uber.org/zap"
)

type SettingHandler struct {
	octantv1alphaconnect.UnimplementedSettingServiceHandler

	builder setting.ManagerBuilder
}

func NewSettingHandler(
	builder setting.ManagerBuilder,
) *SettingHandler {
	return &SettingHandler{
		builder: builder,
	}
}

func (sh *SettingHandler) Update(
	ctx context.Context,
	req *connect.Request[octantv1alpha.UpdateRequest],
	stream *connect.ServerStream[octantv1alpha.UpdateResponse],
) error {
	logger := zap.L().With(
		zap.String("operation", octantv1alphaconnect.SettingServiceUpdateProcedure),
		zap.String("scope", req.Msg.GetScope().String()),
	)

	manager, err := sh.builder.Build(
		ctx,
		req.Msg.GetScope().GetNamespace(),
		req.Msg.GetScope().GetConnectionName(),
		logger,
	)
	if err != nil {
		logger.Error("unable to create a setting manager", zap.Error(err))
		if errors.Is(err, setting.ErrStillUpdating) {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		return connect.NewError(connect.CodeNotFound, err)
	}
	defer sh.builder.Release(ctx, req.Msg.GetScope().GetConnectionName(), manager.ID())

	if conErr := sh.stream(logger, stream, octantv1alpha.UpdateResponse_STATUS_UPDATING); conErr != nil {
		return conErr
	}

	manager = manager.
		SetDatadogURL(req.Msg.GetDatadogUrl()).
		SetDatadogAPIKey(req.Msg.GetDatadogApiKey()).
		SetTelemetryTypes(req.Msg.GetTelemetryTypes())

	if err := manager.Apply(ctx); err != nil {
		logger.Error("unable to apply changes", zap.Error(err))
		return connect.NewError(connect.CodeInternal, err)
	}

	if conErr := sh.stream(logger, stream, octantv1alpha.UpdateResponse_STATUS_UPDATED); conErr != nil {
		return conErr
	}

	results := make(chan setting.SettingUpdateResult)
	go manager.DeployAndWait(ctx, results)

	for result := range results {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, end transfer")
			return nil
		default:
		}

		if result.Err != nil {
			logger.Error("encountered error while deploy and wait", zap.Error(result.Err))
			code := connect.CodeAborted
			if errors.Is(result.Err, setting.ErrDeploy) {
				logger.Error("failed to deploy")
				code = connect.CodeInternal
			}
			return connect.NewError(code, result.Err)
		}

		if conErr := sh.stream(logger, stream, result.Status); conErr != nil {
			return conErr
		}
	}

	return nil
}

func (*SettingHandler) stream(
	logger *zap.Logger,
	stream *connect.ServerStream[octantv1alpha.UpdateResponse],
	status octantv1alpha.UpdateResponse_Status,
) *connect.Error {
	if err := stream.Send(&octantv1alpha.UpdateResponse{
		Status: status,
	}); err != nil {
		logger.Error("Failed to send status", zap.Error(err))
		return connect.NewError(connect.CodeInternal, errors.New("streaming"))
	}
	return nil
}
