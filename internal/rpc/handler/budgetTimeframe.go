package rpchandler

import (
	"context"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	"github.com/mydecisive/octant/internal/budget"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/connection"
	"go.uber.org/zap"
)

type BudgetTimeframeHandler struct {
	budgetv1alphaconnect.UnimplementedTimeframeServiceHandler

	connection connection.Connection[connection.OctantConnectionData]
	retriever  budgetdata.MetricDataRetriever
}

func NewBudgetTimeframeHandler(
	con connection.Connection[connection.OctantConnectionData],
	retriever budgetdata.MetricDataRetriever,
) *BudgetTimeframeHandler {
	return &BudgetTimeframeHandler{
		connection: con,
		retriever:  retriever,
	}
}

func (bth *BudgetTimeframeHandler) TimeframeStatus(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.TimeframeStatusRequest],
) (*connect.Response[budgetv1alpha.TimeframeStatusResponse], error) {
	logger := zap.L().With(zap.String("operation", budgetv1alphaconnect.TimeframeServiceTimeframeStatusProcedure))
	con, err := bth.connection.GetConnectionByName(ctx, connection.Input{
		Logger:         logger,
		ConnectionName: req.Msg.GetConnectionName(),
		Namespace:      req.Msg.GetNamespace(),
	})
	if err != nil {
		logger.Error("failed to get connection data", zap.Error(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if con == nil {
		return connect.NewResponse(&budgetv1alpha.TimeframeStatusResponse{
			Statuses: []*budgetv1alpha.TimeframeStatusResponse_Status{
				{
					Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_24HR,
					Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NO_DATA,
				},
				{
					Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_MTD,
					Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NO_DATA,
				},
				{
					Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_LM,
					Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NO_DATA,
				},
			},
		}), nil
	}

	statuses := &budgetv1alpha.TimeframeStatusResponse{
		Statuses: []*budgetv1alpha.TimeframeStatusResponse_Status{
			{
				Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_24HR,
				Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NOT_ENOUGH,
			},
			{
				Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_MTD,
				Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NOT_ENOUGH,
			},
			{
				Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_LM,
				Status:    budgetv1alpha.TimeframeStatusResponse_CODE_NOT_ENOUGH,
			},
		},
	}

	if ok, err := bth.retriever.RootSpansExist(ctx, req.Msg.GetNamespace()); err != nil {
		logger.Warn("Unable to retrieve root span table status", zap.Error(err))
	} else if ok {
		statuses.Trace = true
	}

	if ok, err := bth.retriever.LogsExist(ctx, req.Msg.GetNamespace()); err != nil {
		logger.Warn("Unable to retrieve logs table status", zap.Error(err))
	} else if ok {
		statuses.Log = true
	}

	for i := range int(budget.ValidTimeframe(con.Created)) {
		statuses.Statuses[i].Status = budgetv1alpha.TimeframeStatusResponse_CODE_OK
	}

	return connect.NewResponse(statuses), nil
}
