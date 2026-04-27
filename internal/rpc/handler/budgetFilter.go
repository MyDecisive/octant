package rpchandler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	budgetfilter "github.com/mydecisive/octant/internal/budget/filter"
	"go.uber.org/zap"
)

type BudgetFilterHandler struct {
	budgetv1alphaconnect.UnimplementedFilterServiceHandler

	setting budgetfilter.SettingController
}

func NewBudgetFilterHandler(setting budgetfilter.SettingController) *BudgetFilterHandler {
	return &BudgetFilterHandler{
		setting: setting,
	}
}

func (bfh *BudgetFilterHandler) GetFilter(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.GetFilterRequest],
) (*connect.Response[budgetv1alpha.GetFilterResponse], error) {
	logger := zap.L().With(zap.String("operation", budgetv1alphaconnect.FilterServiceGetFilterProcedure))
	filter, err := bfh.setting.GetFilter(req.Msg.GetType(), req.Msg.GetNamespace(), req.Msg.GetConnectionName())
	if err != nil {
		logger.Error("failed to get filters", zap.Error(err))
		code := connect.CodeUnknown
		if errors.Is(err, budgetfilter.ErrInvalid) {
			code = connect.CodeInvalidArgument
		}
		if errors.Is(err, budgetfilter.ErrFormat) {
			code = connect.CodeInternal
		}
		if errors.Is(err, budgetfilter.ErrNotFound) {
			code = connect.CodeNotFound
		}
		if errors.Is(err, budgetfilter.ErrStillUpdating) {
			code = connect.CodeUnavailable
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
	logger := zap.L().With(zap.String("operation", budgetv1alphaconnect.FilterServiceUpdateFilterProcedure))
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
			if errors.Is(result.Err, budgetfilter.ErrInvalid) {
				return connect.NewError(connect.CodeInvalidArgument, budgetfilter.ErrInvalid)
			}
			if errors.Is(result.Err, budgetfilter.ErrStillUpdating) {
				return connect.NewError(connect.CodeUnavailable, budgetfilter.ErrStillUpdating)
			}
			if errors.Is(result.Err, budgetfilter.ErrUpdateValue) {
				return connect.NewError(connect.CodeInternal, budgetfilter.ErrUpdateValue)
			}
			if errors.Is(result.Err, budgetfilter.ErrUpdateCollector) {
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
