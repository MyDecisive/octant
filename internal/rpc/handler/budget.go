package rpchandler

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	"github.com/mydecisive/octant/internal/budget"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/connection"
	"github.com/mydecisive/octant/internal/telemetry"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type BudgetHandler struct {
	budgetv1alphaconnect.UnimplementedBudgetServiceHandler

	connection connection.Connection[connection.OctantConnectionData]
	provider   budget.MetricDataProvider
}

func NewBudgetHandler(
	conn connection.Connection[connection.OctantConnectionData],
	provider budget.MetricDataProvider,
) *BudgetHandler {
	return &BudgetHandler{
		connection: conn,
		provider:   provider,
	}
}

func (bh *BudgetHandler) Overall(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.OverallRequest],
) (*connect.Response[budgetv1alpha.OverallResponse], error) {
	logger := zap.L().With(zap.String("operation", budgetv1alphaconnect.BudgetServiceOverallProcedure))
	data, err := bh.provider.GetOverall(ctx, req.Msg.GetTimeframe(), req.Msg.GetNamespace())
	if err != nil {
		logger.Error("failed to get overall budget data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&budgetv1alpha.OverallResponse{
		Data: data,
	}), nil
}

func (bh *BudgetHandler) Log( // nolint: dupl //no its not
	ctx context.Context,
	req *connect.Request[budgetv1alpha.LogRequest],
) (*connect.Response[budgetv1alpha.LogResponse], error) {
	logger := zap.L().With(
		zap.String("operation", budgetv1alphaconnect.BudgetServiceLogProcedure),
		zap.String("connection", req.Msg.GetConnectionName()),
		zap.String("namespace", req.Msg.GetNamespace()),
	)

	if ok, err := bh.isAllowed(
		ctx,
		req.Msg.GetNamespace(),
		req.Msg.GetConnectionName(),
		telemetry.Logs,
	); err != nil {
		logger.Error("failed to get connection data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !ok {
		logger.Error("connection does not support log type", zap.Error(errors.New("log telemetry type not vailable")))
		return nil, connect.NewError(connect.CodeNotFound, errors.New("log telemetry type not available"))
	}

	data, nextPage, errGet := bh.provider.GetLogs(ctx, budgetdata.MetricDataInput{
		Timeframe: req.Msg.GetTimeframe(),
		Size:      req.Msg.GetSize(),
		PageToken: req.Msg.GetPageToken(),
		Search:    req.Msg.GetSearch(),
		Namespace: req.Msg.GetNamespace(),
	})
	if errGet != nil {
		logger.Error("failed to get log data", zap.Error(errGet))
		return nil, connect.NewError(connect.CodeInternal, errGet)
	}
	return connect.NewResponse(&budgetv1alpha.LogResponse{
		Data:          data,
		NextPageToken: nextPage,
	}), nil
}

func (bh *BudgetHandler) Trace( // nolint: dupl //no its not
	ctx context.Context,
	req *connect.Request[budgetv1alpha.TraceRequest],
) (*connect.Response[budgetv1alpha.TraceResponse], error) {
	logger := zap.L().With(
		zap.String("operation", budgetv1alphaconnect.BudgetServiceTraceProcedure),
		zap.String("connection", req.Msg.GetConnectionName()),
		zap.String("namespace", req.Msg.GetNamespace()),
	)

	if ok, err := bh.isAllowed(
		ctx,
		req.Msg.GetNamespace(),
		req.Msg.GetConnectionName(),
		telemetry.Traces,
	); err != nil {
		logger.Error("failed to get connection data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if !ok {
		logger.Error("connection does not support trace type", zap.Error(errors.New("trace telemetry type not available")))
		return nil, connect.NewError(connect.CodeNotFound, errors.New("trace telemetry type not available"))
	}

	data, nextPage, err := bh.provider.GetSpans(ctx, budgetdata.MetricDataInput{
		Timeframe: req.Msg.GetTimeframe(),
		Size:      req.Msg.GetSize(),
		PageToken: req.Msg.GetPageToken(),
		Search:    req.Msg.GetSearch(),
		Namespace: req.Msg.GetNamespace(),
	})
	if err != nil {
		logger.Error("failed to get trace data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&budgetv1alpha.TraceResponse{
		Data:          data,
		NextPageToken: nextPage,
	}), nil
}

// isAllowed returns true if the given connection allows the given MLT type.
func (bh *BudgetHandler) isAllowed(
	ctx context.Context,
	namespace string,
	conn string,
	mlt telemetry.MLT,
) (bool, error) {
	con, err := bh.connection.GetConnectionByName(ctx, namespace, conn)
	if err != nil {
		return false, err
	}

	if con == nil {
		return false, nil
	}
	return lo.Contains(con.TelemetryTypes, mlt), nil
}
