package rpchandler

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	"github.com/mydecisive/octant/internal/connection"
	connectionmock "github.com/mydecisive/octant/internal/mock/connection"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBudgetTimeframeHandler_TimeframeStatus(t *testing.T) {
	t.Parallel()

	var input *budgetv1alpha.TimeframeStatusRequest
	require.NoError(t, faker.FakeData(&input, options.WithRandomMapAndSliceMaxSize(1)))

	cases := []struct {
		des      string
		in       time.Time
		expected budgetv1alpha.Timeframe
	}{
		{
			"success not enough",
			time.Now().Add(-(1 * time.Hour)),
			budgetv1alpha.Timeframe_TIMEFRAME_UNSPECIFIED,
		},
		{
			"success up to 24h",
			time.Now().Add(-(24 * time.Hour)),
			budgetv1alpha.Timeframe_TIMEFRAME_24HR,
		},
		{
			"success up to month to date",
			time.Now().Add(-(730 * time.Hour)),
			budgetv1alpha.Timeframe_TIMEFRAME_MTD,
		},
		{
			"success all",
			time.Now().Add(-(1460 * time.Hour)),
			budgetv1alpha.Timeframe_TIMEFRAME_LM,
		},
	}
	for _, tt := range cases {
		t.Run(tt.des, func(t *testing.T) {
			t.Parallel()

			mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
			mockConn.EXPECT().GetConnectionByName(
				mock.Anything,
				input.GetNamespace(),
				input.GetConnectionName(),
			).Return(&connection.OctantConnectionData{
				Created: tt.in,
			}, nil).Once()

			target := NewBudgetTimeframeHandler(mockConn)

			actual, err := target.TimeframeStatus(t.Context(), connect.NewRequest(input))
			require.NoError(t, err)

			assert.Len(t, actual.Msg.GetStatuses(), 3)
			for i := range 3 {
				if i < int(tt.expected) {
					assert.Equal(t, budgetv1alpha.TimeframeStatusResponse_CODE_OK, actual.Msg.GetStatuses()[i].GetStatus())
				} else {
					assert.Equal(t, budgetv1alpha.TimeframeStatusResponse_CODE_NOT_ENOUGH, actual.Msg.GetStatuses()[i].GetStatus())
				}
			}
		})
	}

	t.Run("success no data", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			input.GetNamespace(),
			input.GetConnectionName(),
		).Return(nil, nil).Once()

		target := NewBudgetTimeframeHandler(mockConn)

		actual, err := target.TimeframeStatus(t.Context(), connect.NewRequest(input))
		require.NoError(t, err)

		assert.Len(t, actual.Msg.GetStatuses(), 3)
		for i := range 3 {
			assert.Equal(t, budgetv1alpha.TimeframeStatusResponse_CODE_NO_DATA, actual.Msg.GetStatuses()[i].GetStatus())
		}
	})

	t.Run("err", func(t *testing.T) {
		t.Parallel()

		mockConn := connectionmock.NewMockConnection[connection.OctantConnectionData](t)
		mockConn.EXPECT().GetConnectionByName(
			mock.Anything,
			input.GetNamespace(),
			input.GetConnectionName(),
		).Return(nil, assert.AnError).Once()

		target := NewBudgetTimeframeHandler(mockConn)

		actual, err := target.TimeframeStatus(t.Context(), connect.NewRequest(input))
		require.Error(t, err)

		assert.Nil(t, actual)
	})
}
