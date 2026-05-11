package rpchandler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	budgetfilter "github.com/mydecisive/octant/internal/budget/filter"
	"github.com/mydecisive/octant/internal/config"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type BudgetFilterHandler struct {
	budgetv1alphaconnect.UnimplementedFilterServiceHandler

	currentNamespace string
	connection       connection.Connection[connection.OctantConnectionData]
	setting          budgetfilter.SettingController
}

func NewBudgetFilterHandler(
	co config.Configuration,
	conn connection.Connection[connection.OctantConnectionData],
	setting budgetfilter.SettingController,
) *BudgetFilterHandler {
	return &BudgetFilterHandler{
		currentNamespace: co.CurrentNamespace,
		connection:       conn,
		setting:          setting,
	}
}

func (bfh *BudgetFilterHandler) GetFilter(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.GetFilterRequest],
) (*connect.Response[budgetv1alpha.GetFilterResponse], error) {
	logger := zap.L().With(
		zap.String("operation", budgetv1alphaconnect.FilterServiceGetFilterProcedure),
		zap.String("type", req.Msg.GetType().String()),
		zap.String("namespace", req.Msg.GetNamespace()),
		zap.String("connection", req.Msg.GetConnectionName()),
	)

	if ok, err := bfh.isAllowed(
		ctx,
		bfh.currentNamespace,
		req.Msg.GetConnectionName(),
		req.Msg.GetType(),
	); err != nil {
		logger.Error("failed to get connection data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !ok {
		logger.Error("connection does not support given type", zap.Error(err))
		return nil, connect.NewError(connect.CodeNotFound, errors.New("telemetry type not available"))
	}

	filter, err := bfh.setting.GetFilter(req.Msg.GetType(), req.Msg.GetNamespace(), req.Msg.GetConnectionName())
	if err != nil {
		logger.Error("failed to get filters", zap.Error(err))
		code := connect.CodeUnknown
		// nolint: gocritic // checking error type must use errors.Is
		if errors.Is(err, budgetfilter.ErrStillUpdating) {
			code = connect.CodeUnavailable
		} else if errors.Is(err, budgetfilter.ErrInvalid) {
			code = connect.CodeInvalidArgument
		} else if errors.Is(err, budgetfilter.ErrFormat) ||
			errors.Is(err, budgetfilter.ErrNotFound) {
			code = connect.CodeInternal
		}
		return nil, connect.NewError(code, err)
	}
	filter.Type = req.Msg.GetType()
	return connect.NewResponse(&budgetv1alpha.GetFilterResponse{
		Data: filter,
	}), nil
}

func (bfh *BudgetFilterHandler) UpdateFilter(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.UpdateFilterRequest],
	stream *connect.ServerStream[budgetv1alpha.UpdateFilterResponse],
) error {
	logger := zap.L().With(
		zap.String("operation", budgetv1alphaconnect.FilterServiceUpdateFilterProcedure),
		zap.String("type", req.Msg.GetData().GetType().String()),
		zap.String("namespace", req.Msg.GetNamespace()),
		zap.String("connection", req.Msg.GetConnectionName()),
	)

	if ok, err := bfh.isAllowed(
		ctx,
		bfh.currentNamespace,
		req.Msg.GetConnectionName(),
		req.Msg.GetData().GetType(),
	); err != nil {
		logger.Error("failed to get connection data", zap.Error(err))
		return connect.NewError(connect.CodeInternal, err)
	} else if !ok {
		logger.Error("connection does not support given type", zap.Error(err))
		return connect.NewError(connect.CodeNotFound, errors.New("telemetry type not available"))
	}

	results := make(chan budgetfilter.UpdateFilterResult)

	go func() {
		bfh.setting.UpdateFilter(ctx, req.Msg.GetNamespace(), req.Msg.GetConnectionName(), req.Msg.GetData(), results)
	}()

	for result := range results {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, end transfer")
			return nil
		default:
		}

		if result.Err != nil {
			logger.Error("failed to update filters", zap.Error(result.Err))
			// nolint: gocritic // checking error type must use errors.Is
			if errors.Is(result.Err, budgetfilter.ErrStillUpdating) {
				return connect.NewError(connect.CodeUnavailable, budgetfilter.ErrStillUpdating)
			} else if errors.Is(result.Err, budgetfilter.ErrInvalid) {
				return connect.NewError(connect.CodeInvalidArgument, budgetfilter.ErrInvalid)
			} else if errors.Is(result.Err, budgetfilter.ErrUpdateValue) {
				return connect.NewError(connect.CodeInternal, budgetfilter.ErrUpdateValue)
			} else if errors.Is(result.Err, budgetfilter.ErrUpdateCollector) {
				return connect.NewError(connect.CodeInternal, budgetfilter.ErrUpdateCollector)
			}
			return connect.NewError(connect.CodeUnknown, result.Err)
		}

		if err := stream.Send(&budgetv1alpha.UpdateFilterResponse{
			Status: result.Status,
		}); err != nil {
			logger.Error("Failed to send status", zap.Error(err))
			return connect.NewError(connect.CodeInternal, errors.New("streaming"))
		}
	}

	return nil
}

// isAllowed returns true if the given connection allows the given MLT type.
func (bfh *BudgetFilterHandler) isAllowed(
	ctx context.Context,
	namespace string,
	conn string,
	mlt budgetv1alpha.FilterType,
) (bool, error) {
	con, err := bfh.connection.GetConnectionByName(ctx, namespace, conn)
	if err != nil {
		return false, err
	}
	if con == nil {
		return false, nil
	}

	mltType := telemetry.Logs
	if mlt == budgetv1alpha.FilterType_FILTER_TYPE_TRACE {
		mltType = telemetry.Traces
	}

	return lo.Contains(con.TelemetryTypes, mltType), nil
}
