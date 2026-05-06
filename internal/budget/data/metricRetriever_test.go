package budgetdata

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/go-faker/faker/v4"
	budgetdb "github.com/mydecisive/octant/internal/budget/data/db"
	budgetdbmock "github.com/mydecisive/octant/internal/mock/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGreptimeDataRetriever_GetOverall(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	timeframe := budgetv1alpha.Timeframe_TIMEFRAME_MTD
	expectedLogRecSQL := "SELECT SUM(bytes_received_by_service_total.greptime_value / ?) FROM public.bytes_received_by_service_total WHERE CAST(bytes_received_by_service_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 730 HOUR) AS FLOAT);" //nolint:lll
	expectedLogSentSQL := "SELECT SUM(bytes_sent_by_service_total.greptime_value / ?) FROM public.bytes_sent_by_service_total WHERE CAST(bytes_sent_by_service_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 730 HOUR) AS FLOAT);"            //nolint:lll
	expectedSpanRecSQL := "SELECT SUM(received_span_root_count_total.greptime_value / ?) FROM public.received_span_root_count_total WHERE CAST(received_span_root_count_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 730 HOUR) AS FLOAT);"   //nolint:lll
	expectedSpanSentSQL := "SELECT SUM(sent_span_count_total.greptime_value / ?) FROM public.sent_span_count_total WHERE CAST(sent_span_count_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 730 HOUR) AS FLOAT);"                             //nolint:lll

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var expected *Overall
		require.NoError(t, faker.FakeData(&expected))

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedLogRecSQL).
			WithArgs(toGB).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).
				AddRow(expected.LogReceived))

		dbmock.ExpectQuery(expectedLogSentSQL).
			WithArgs(toGB).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).
				AddRow(expected.LogSend))

		dbmock.ExpectQuery(expectedSpanRecSQL).
			WithArgs(toMil).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).
				AddRow(expected.SpanReceived))

		dbmock.ExpectQuery(expectedSpanSentSQL).
			WithArgs(toMil).
			WillReturnRows(sqlmock.NewRows([]string{"total"}).
				AddRow(expected.SpanSend))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetOverall(t.Context(), timeframe, namespace)
		require.NoError(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("success no data", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedLogRecSQL).
			WithArgs(toGB).WillReturnError(assert.AnError)

		dbmock.ExpectQuery(expectedLogSentSQL).
			WithArgs(toGB).WillReturnError(assert.AnError)

		dbmock.ExpectQuery(expectedSpanRecSQL).
			WithArgs(toMil).WillReturnError(assert.AnError)

		dbmock.ExpectQuery(expectedSpanSentSQL).
			WithArgs(toMil).WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetOverall(t.Context(), timeframe, namespace)
		require.NoError(t, err)
		assert.Equal(t, &Overall{}, actual)
	})

	t.Run("err build", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetOverall(t.Context(), timeframe, namespace)
		assert.Nil(t, actual)
		assert.Error(t, err)
	})
}

func TestGreptimeDataRetriever_GetTotalLog(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	timeframe := budgetv1alpha.Timeframe_TIMEFRAME_MTD
	expectedSQL := "SELECT SUM(bytes_sent_by_service_total.greptime_value / ?) FROM public.bytes_sent_by_service_total WHERE CAST(bytes_sent_by_service_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 730 HOUR) AS FLOAT);" //nolint:lll

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		expected := faker.Latitude()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WithArgs(toGB).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(expected))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetTotalLog(t.Context(), timeframe, namespace)
		require.NoError(t, err)
		assert.InDelta(t, expected, actual, 0.01)
	})

	t.Run("err query", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WithArgs(toGB).WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetTotalLog(t.Context(), timeframe, namespace)
		assert.InDelta(t, -1, actual, 0.01)
		assert.Error(t, err)
	})

	t.Run("err builder", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.GetTotalLog(t.Context(), timeframe, namespace)
		assert.InDelta(t, -1, actual, 0.01)
		assert.Error(t, err)
	})
}

func TestGreptimeDataRetriever_GetLogs(t *testing.T) {
	t.Parallel()

	input := MetricDataInput{
		Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_LM,
		Size:      1,
		Namespace: faker.Word(),
	}

	expectedSQL := "SELECT bytes_sent_by_service_total.service AS \"log.name\", SUM(bytes_sent_by_service_total.greptime_value / ?) AS \"log.amount\" FROM public.bytes_sent_by_service_total WHERE CAST(bytes_sent_by_service_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 1460 HOUR) AS FLOAT) GROUP BY bytes_sent_by_service_total.service ORDER BY `log.amount` DESC LIMIT ?;" //nolint:lll

	t.Run("success empty", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toGB, input.Size+1).
			WillReturnRows(sqlmock.NewRows([]string{"log.name", "log.amount"}))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), input)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Empty(t, actual)
	})

	t.Run("success no next", func(t *testing.T) {
		t.Parallel()
		var expected Log
		require.NoError(t, faker.FakeData(&expected))

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toGB, input.Size+1).
			WillReturnRows(sqlmock.NewRows([]string{"log.name", "log.amount"}).AddRow(expected.Name, expected.Amount))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), input)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success with next", func(t *testing.T) {
		t.Parallel()
		var expected Log
		require.NoError(t, faker.FakeData(&expected))
		expectedNext := faker.Word()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toGB, input.Size+1).
			WillReturnRows(
				sqlmock.NewRows([]string{"log.name", "log.amount"}).
					AddRow(expected.Name, expected.Amount).
					AddRow(expectedNext, expected.Amount))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), input)
		require.NoError(t, err)
		assert.Equal(t, expectedNext, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success search", func(t *testing.T) {
		t.Parallel()
		inputSearch := MetricDataInput{
			Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_LM,
			Size:      1,
			Namespace: faker.Word(),
			Search:    faker.Word(),
		}

		expectedSearchSQL := "SELECT bytes_sent_by_service_total.service AS \"log.name\", SUM(bytes_sent_by_service_total.greptime_value / ?) AS \"log.amount\" FROM public.bytes_sent_by_service_total WHERE (CAST(bytes_sent_by_service_total.greptime_timestamp AS FLOAT) >= CAST((NOW() - INTERVAL 1460 HOUR) AS FLOAT)) AND (bytes_sent_by_service_total.service LIKE ?) GROUP BY bytes_sent_by_service_total.service ORDER BY `log.amount` DESC LIMIT ?;" //nolint:lll

		var expected Log
		require.NoError(t, faker.FakeData(&expected))

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, inputSearch.Namespace).Return(&budgetdb.Database{
			Namespace: inputSearch.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSearchSQL).
			WithArgs(toGB, "%"+inputSearch.Search+"%", inputSearch.Size+1).
			WillReturnRows(sqlmock.NewRows([]string{"log.name", "log.amount"}).AddRow(expected.Name, expected.Amount))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), inputSearch)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("err query", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toGB, input.Size+1).
			WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), input)
		assert.Empty(t, next)
		assert.Empty(t, actual)
		assert.Error(t, err)
	})

	t.Run("err build", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetLogs(t.Context(), input)
		assert.Empty(t, next)
		assert.Empty(t, actual)
		assert.Error(t, err)
	})
}

func TestGreptimeDataRetriever_GetRootSpans(t *testing.T) {
	t.Parallel()

	input := MetricDataInput{
		Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_24HR,
		Size:      1,
		Namespace: faker.Word(),
	}

	expectedSQL := "SELECT trace_root_topology_1m.root_id AS \"root_span.name\", SUM(CAST(trace_root_topology_1m.trace_count AS FLOAT) / ?) AS \"root_span.count\", (uddsketch_calc(0.50, uddsketch_merge(128, 0.01, breadth_sketch))) AS \"root_span.breadth\", (uddsketch_calc(0.50, uddsketch_merge(128, 0.01, depth_sketch))) AS \"root_span.depth\", ((uddsketch_calc(0.50, uddsketch_merge(128, 0.01, duration_sketch))) / ?) AS \"root_span.invocation\" FROM public.trace_root_topology_1m WHERE CAST(trace_root_topology_1m.time_window AS FLOAT) >= CAST((NOW() - INTERVAL 24 HOUR) AS FLOAT) GROUP BY trace_root_topology_1m.root_id ORDER BY `root_span.count` DESC LIMIT ?;" //nolint:lll

	t.Run("success empty", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toMil, toMil, input.Size+1).
			WillReturnRows(
				sqlmock.NewRows([]string{
					"root_span.name", "root_span.count", "root_span.breadth", "root_span.depth", "root_span.invocation",
				}))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), input)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Empty(t, actual)
	})

	t.Run("success no next", func(t *testing.T) {
		t.Parallel()
		var expected RootSpan
		require.NoError(t, faker.FakeData(&expected))

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toMil, toMil, input.Size+1).
			WillReturnRows(
				sqlmock.NewRows([]string{
					"root_span.name", "root_span.count", "root_span.breadth", "root_span.depth", "root_span.invocation",
				}).AddRow(
					expected.Name, expected.Count, expected.Breadth, expected.Depth, expected.Invocation,
				))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), input)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success with next", func(t *testing.T) {
		t.Parallel()
		var expected RootSpan
		require.NoError(t, faker.FakeData(&expected))
		expectedNext := faker.Word()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toMil, toMil, input.Size+1).
			WillReturnRows(
				sqlmock.NewRows([]string{
					"root_span.name", "root_span.count", "root_span.breadth", "root_span.depth", "root_span.invocation",
				}).AddRow(
					expected.Name, expected.Count, expected.Breadth, expected.Depth, expected.Invocation,
				).AddRow(
					expectedNext, expected.Count, expected.Breadth, expected.Depth, expected.Invocation,
				))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), input)
		require.NoError(t, err)
		assert.Equal(t, expectedNext, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("success search", func(t *testing.T) {
		t.Parallel()
		inputSearch := MetricDataInput{
			Timeframe: budgetv1alpha.Timeframe_TIMEFRAME_LM,
			Size:      1,
			Namespace: faker.Word(),
			Search:    faker.Word(),
		}

		expectedSearchSQL := "SELECT trace_root_topology_1m.root_id AS \"root_span.name\", SUM(CAST(trace_root_topology_1m.trace_count AS FLOAT) / ?) AS \"root_span.count\", (uddsketch_calc(0.50, uddsketch_merge(128, 0.01, breadth_sketch))) AS \"root_span.breadth\", (uddsketch_calc(0.50, uddsketch_merge(128, 0.01, depth_sketch))) AS \"root_span.depth\", ((uddsketch_calc(0.50, uddsketch_merge(128, 0.01, duration_sketch))) / ?) AS \"root_span.invocation\" FROM public.trace_root_topology_1m WHERE (CAST(trace_root_topology_1m.time_window AS FLOAT) >= CAST((NOW() - INTERVAL 1460 HOUR) AS FLOAT)) AND (trace_root_topology_1m.root_id LIKE ?) GROUP BY trace_root_topology_1m.root_id ORDER BY `root_span.count` DESC LIMIT ?;" //nolint:lll

		var expected RootSpan
		require.NoError(t, faker.FakeData(&expected))

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, inputSearch.Namespace).Return(&budgetdb.Database{
			Namespace: inputSearch.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSearchSQL).
			WithArgs(toMil, toMil, "%"+inputSearch.Search+"%", inputSearch.Size+1).
			WillReturnRows(sqlmock.NewRows([]string{
				"root_span.name", "root_span.count", "root_span.breadth", "root_span.depth", "root_span.invocation",
			}).AddRow(
				expected.Name, expected.Count, expected.Breadth, expected.Depth, expected.Invocation,
			))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), inputSearch)
		require.NoError(t, err)
		assert.Empty(t, next)
		assert.Len(t, actual, 1)
		assert.Equal(t, expected, actual[0])
	})

	t.Run("err query", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(&budgetdb.Database{
			Namespace: input.Namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).
			WithArgs(toMil, toMil, input.Size+1).
			WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), input)
		assert.Empty(t, next)
		assert.Empty(t, actual)
		assert.Error(t, err)
	})

	t.Run("err build", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, input.Namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, next, err := target.GetRootSpans(t.Context(), input)
		assert.Empty(t, next)
		assert.Empty(t, actual)
		assert.Error(t, err)
	})
}

func TestGreptimeDataRetriever_RootSpansExist(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	expectedSQL := "SHOW TABLES LIKE 'trace_root_topology_1m';"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WillReturnRows(sqlmock.NewRows([]string{faker.Word()}).AddRow(faker.Word()))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.RootSpansExist(t.Context(), namespace)
		require.NoError(t, err)
		assert.True(t, actual)
	})

	t.Run("success no table", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WillReturnRows(sqlmock.NewRows([]string{faker.Word()}))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.RootSpansExist(t.Context(), namespace)
		require.NoError(t, err)
		assert.False(t, actual)
	})

	t.Run("err query", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WithArgs(toGB).WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.RootSpansExist(t.Context(), namespace)
		assert.False(t, actual)
		assert.Error(t, err)
	})

	t.Run("err builder", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.RootSpansExist(t.Context(), namespace)
		assert.False(t, actual)
		assert.Error(t, err)
	})
}

func TestGreptimeDataRetriever_LogsExist(t *testing.T) {
	t.Parallel()

	namespace := faker.Word()
	expectedSQL := "SHOW TABLES LIKE 'bytes_sent_by_service_total';"

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WillReturnRows(sqlmock.NewRows([]string{faker.Word()}).AddRow(faker.Word()))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.LogsExist(t.Context(), namespace)
		require.NoError(t, err)
		assert.True(t, actual)
	})

	t.Run("success no table", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WillReturnRows(sqlmock.NewRows([]string{faker.Word()}))

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.LogsExist(t.Context(), namespace)
		require.NoError(t, err)
		assert.False(t, actual)
	})

	t.Run("err query", func(t *testing.T) {
		t.Parallel()

		fakedb, dbmock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer fakedb.Close() //nolint:errcheck

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(&budgetdb.Database{
			Namespace: namespace,
			DB:        fakedb,
		}, nil).Once()

		dbmock.ExpectQuery(expectedSQL).WithArgs(toGB).WillReturnError(assert.AnError)

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.LogsExist(t.Context(), namespace)
		assert.False(t, actual)
		assert.Error(t, err)
	})

	t.Run("err builder", func(t *testing.T) {
		t.Parallel()

		mockBuilder := budgetdbmock.NewMockDatabaseAccessBuilder(t)
		mockBuilder.EXPECT().Build(mock.Anything, namespace).Return(nil, assert.AnError).Once()

		target := NewGreptimeDataRetriever(mockBuilder)
		actual, err := target.LogsExist(t.Context(), namespace)
		assert.False(t, actual)
		assert.Error(t, err)
	})
}
