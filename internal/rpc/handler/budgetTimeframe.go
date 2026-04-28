package rpchandler

import (
	"context"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha/budgetv1alphaconnect"
	"github.com/mydecisive/octant/internal/budget"
	"github.com/mydecisive/octant/internal/connection"
	"go.uber.org/zap"
)

type BudgetTimeframeHandler struct {
	budgetv1alphaconnect.UnimplementedTimeframeServiceHandler

	connection connection.Connection[connection.OctantConnectionData]
}

func NewBudgetTimeframeHandler(con connection.Connection[connection.OctantConnectionData]) *BudgetTimeframeHandler {
	return &BudgetTimeframeHandler{
		connection: con,
	}
}

func (bth *BudgetTimeframeHandler) TimeframeStatus(
	ctx context.Context,
	req *connect.Request[budgetv1alpha.TimeframeStatusRequest],
) (*connect.Response[budgetv1alpha.TimeframeStatusResponse], error) {
	logger := zap.L().With(zap.String("operation", budgetv1alphaconnect.TimeframeServiceTimeframeStatusProcedure))
	con, err := bth.connection.GetConnectionByName(ctx, req.Msg.GetNamespace(), req.Msg.GetConnectionName())
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
	for i := range int(budget.ValidTimeframe(con.Created)) {
		statuses.Statuses[i].Status = budgetv1alpha.TimeframeStatusResponse_CODE_OK
	}
	return connect.NewResponse(statuses), nil
}
