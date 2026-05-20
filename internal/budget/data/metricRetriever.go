package budgetdata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	. "github.com/go-jet/jet/v2/mysql" //nolint
	budgetdb "github.com/mydecisive/octant/internal/budget/data/db"
	. "github.com/mydecisive/octant/internal/budget/data/db/public/table" //nolint
	"go.uber.org/zap"
)

const (
	toGB  = 1073741824.0
	toMil = 1000000.0

	uddsketchCalcFormatter = "uddsketch_calc(0.50, uddsketch_merge(128, 0.01, %s))"
	showTableFormatter     = "SHOW TABLES LIKE '%s'"
)

const (
	DayInHR       = 24   // 1 day
	MonthInHR     = 730  // 30 days (i.e., closest approx. to a month)
	LastMonthInHR = 1460 // 60 days (i.e., closest approx. to 2 month)
)

var (
	ErrQuery      = errors.New("query error")
	ErrConnection = errors.New("connection error")
)

// MetricDataRetriever is used to retrieve metric data from the data store.
type MetricDataRetriever interface {
	// GetOverall returns the overall summary of the log and span data for the given timeframe.
	GetOverall(ctx context.Context, timeframe budgetv1alpha.Timeframe, namespace string) (*Overall, error)
	// GetTotalLog returns total amount of log data sent.
	GetTotalLog(ctx context.Context, timeframe budgetv1alpha.Timeframe, namespace string) (float64, error)
	// GetLogs returns the list of log data that matches the given inputs.
	GetLogs(ctx context.Context, input MetricDataInput) ([]Log, string, error)
	// GetRootSpans returns the list of root span data that matches the given inputs.
	GetRootSpans(ctx context.Context, input MetricDataInput) ([]RootSpan, string, error)
	// RootSpansExist returns true if root span table exists.
	RootSpansExist(ctx context.Context, namespace string) (bool, error)
	// LogsExist returns true if log table exists.
	LogsExist(ctx context.Context, namespace string) (bool, error)
}

// Ensure GreptimeDataRetriever implements MetricDataRetriever.
var _ MetricDataRetriever = &GreptimeDataRetriever{}

type GreptimeDataRetriever struct {
	builder budgetdb.DatabaseAccessBuilder
}

// NewGreptimeDataRetriever creates a new instance of GreptimeDataRetriever.
func NewGreptimeDataRetriever(
	builder budgetdb.DatabaseAccessBuilder,
) *GreptimeDataRetriever {
	return &GreptimeDataRetriever{
		builder: builder,
	}
}

// GetOverall returns the overall summary of the log and span data for the given timeframe.
func (gdr *GreptimeDataRetriever) GetOverall(
	ctx context.Context,
	timeframe budgetv1alpha.Timeframe,
	namespace string,
) (*Overall, error) {
	conn, err := gdr.builder.Build(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	logRec, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		BytesReceivedByServiceTotal,
		BytesReceivedByServiceTotal.GreptimeValue,
		BytesReceivedByServiceTotal.GreptimeTimestamp,
		toGB,
	)
	if err != nil {
		zap.L().Warn("Encountered errors while retrieving total log data amount received", zap.Error(err))
		logRec = 0
	}

	logSent, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		BytesSentByServiceTotal,
		BytesSentByServiceTotal.GreptimeValue,
		BytesSentByServiceTotal.GreptimeTimestamp,
		toGB,
	)
	if err != nil {
		zap.L().Warn("Encountered errors while retrieving total log data amount sent", zap.Error(err))
		logSent = 0
	}

	spanRec, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		ReceivedSpanRootCountTotal,
		ReceivedSpanRootCountTotal.GreptimeValue,
		ReceivedSpanRootCountTotal.GreptimeTimestamp,
		toMil,
	)
	if err != nil {
		zap.L().Warn("Encountered errors while retrieving total received span counts", zap.Error(err))
		spanRec = 0
	}

	spanSent, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		SentSpanCountTotal,
		SentSpanCountTotal.GreptimeValue,
		SentSpanCountTotal.GreptimeTimestamp,
		toMil,
	)
	if err != nil {
		zap.L().Warn("Encountered errors while retrieving total sent span counts", zap.Error(err))
		spanSent = 0
	}

	return &Overall{
		LogReceived:  logRec,
		LogSend:      logSent,
		SpanReceived: spanRec,
		SpanSend:     spanSent,
	}, nil //nolint
}

// GetTotalLog returns total amount of log data sent.
func (gdr *GreptimeDataRetriever) GetTotalLog(
	ctx context.Context,
	timeframe budgetv1alpha.Timeframe,
	namespace string,
) (float64, error) {
	table := BytesSentByServiceTotal

	conn, err := gdr.builder.Build(ctx, namespace)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	return gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		table,
		table.GreptimeValue,
		table.GreptimeTimestamp,
		toGB,
	)
}

// GetLogs returns the list of log data that matches the given inputs.
func (gdr *GreptimeDataRetriever) GetLogs(
	ctx context.Context,
	input MetricDataInput,
) ([]Log, string, error) {
	table := BytesSentByServiceTotal

	conn, err := gdr.builder.Build(ctx, input.Namespace)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrConnection, err)
	}

	where := gdr.timeRangeExpression(input.Timeframe, table.GreptimeTimestamp)
	if input.Search != "" {
		where = where.AND(table.Service.LIKE(String("%" + input.Search + "%")))
	}
	stmt := SELECT(
		table.Service.AS("log.name"),
		SUM(table.GreptimeValue.DIV(Float(toGB))).AS("log.amount"),
	).FROM(table).
		WHERE(where).
		GROUP_BY(table.Service).
		ORDER_BY(Raw("`log.amount` DESC")).
		LIMIT(int64(input.Size + 1))

	var result []Log
	if err := stmt.QueryContext(ctx, conn.DB, &result); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrQuery, err)
	}

	next := ""
	if len(result) > int(input.Size) {
		next = result[int(input.Size)].Name
		result = result[:len(result)-1]
	}
	return result, next, nil
}

// GetRootSpans returns the list of root span data that matches the given inputs.
func (gdr *GreptimeDataRetriever) GetRootSpans(
	ctx context.Context,
	input MetricDataInput,
) ([]RootSpan, string, error) {
	table := TraceRootTopology1m

	conn, err := gdr.builder.Build(ctx, input.Namespace)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrConnection, err)
	}

	where := gdr.timeRangeExpression(input.Timeframe, table.TimeWindow)
	if input.Search != "" {
		where = where.AND(table.RootID.LIKE(String("%" + input.Search + "%")))
	}
	stmt := SELECT(
		table.RootID.AS("root_span.name"),
		SUM(CAST(table.TraceCount).AS_FLOAT().DIV(Float(toMil))).AS("root_span.count"),
		RawFloat(gdr.uddsketchCalc(table.BreadthSketch)).AS("root_span.breadth"),
		RawFloat(gdr.uddsketchCalc(table.DepthSketch)).AS("root_span.depth"),
		RawFloat(gdr.uddsketchCalc(table.DurationSketch)).DIV(Float(toMil)).
			AS("root_span.invocation"),
	).FROM(table).
		WHERE(where).
		GROUP_BY(table.RootID).
		ORDER_BY(Raw("`root_span.count` DESC")).
		LIMIT(int64(input.Size + 1))

	var result []RootSpan
	if err := stmt.QueryContext(ctx, conn.DB, &result); err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrQuery, err)
	}

	next := ""
	if len(result) > int(input.Size) {
		next = result[int(input.Size)].Name
		result = result[:len(result)-1]
	}
	return result, next, nil
}

// RootSpansExist returns true if root span table exists.
func (gdr *GreptimeDataRetriever) RootSpansExist(ctx context.Context, namespace string) (bool, error) {
	conn, err := gdr.builder.Build(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	return gdr.tableExists(ctx, conn.DB, TraceRootTopology1m)
}

// LogsExist returns true if log table exists.
func (gdr *GreptimeDataRetriever) LogsExist(ctx context.Context, namespace string) (bool, error) {
	conn, err := gdr.builder.Build(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	return gdr.tableExists(ctx, conn.DB, BytesSentByServiceTotal)
}

// getTotal returns total sum of the valueCol divided by the divisor.
func (gdr *GreptimeDataRetriever) getTotal(
	ctx context.Context,
	db *sql.DB,
	timeframe budgetv1alpha.Timeframe,
	table ReadableTable,
	valueCol ColumnFloat,
	timestampCol ColumnString,
	divisor float64,
) (float64, error) {
	stmt := SELECT(
		SUM(valueCol.DIV(Float(divisor))),
	).FROM(table).WHERE(gdr.timeRangeExpression(timeframe, timestampCol))

	var result []float64
	if err := stmt.QueryContext(ctx, db, &result); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrQuery, err)
	}
	if len(result) > 0 {
		return result[0], nil
	}
	return 0, nil
}

// tableExists returns true of the given table exists in greptimedb.
func (*GreptimeDataRetriever) tableExists(
	ctx context.Context,
	db *sql.DB,
	table Table,
) (bool, error) {
	stmt := RawStatement(fmt.Sprintf(showTableFormatter, table.TableName()))

	var res []string
	if err := stmt.QueryContext(ctx, db, &res); err != nil {
		return false, fmt.Errorf("%w: %w", ErrQuery, err)
	}

	return len(res) > 0, nil
}

// timeRangeExpression generates a bool expression that can be used
// to only retrieve data within the given timeframe.
func (gdr *GreptimeDataRetriever) timeRangeExpression( //nolint:ireturn
	timeframe budgetv1alpha.Timeframe,
	timestampCol ColumnString,
) BoolExpression {
	timeInt := gdr.toHr(timeframe)
	if timeframe < budgetv1alpha.Timeframe_TIMEFRAME_LM {
		return CAST(timestampCol).AS_DATETIME().GT_EQ(
			NOW().SUB(INTERVAL(timeInt, HOUR)),
		)
	}
	return CAST(timestampCol).AS_DATETIME().BETWEEN(
		NOW().SUB(INTERVAL(timeInt, HOUR)),
		NOW().SUB(INTERVAL(gdr.toHr(budgetv1alpha.Timeframe_TIMEFRAME_MTD), HOUR)),
	)
}

// toHr converts timeframe enum to number of hours.
func (*GreptimeDataRetriever) toHr(timeframe budgetv1alpha.Timeframe) int {
	switch timeframe {
	case budgetv1alpha.Timeframe_TIMEFRAME_24HR:
		return DayInHR
	case budgetv1alpha.Timeframe_TIMEFRAME_MTD:
		return MonthInHR
	default:
		return LastMonthInHR
	}
}

// uddsketchCalc returns the saw query to perform the uddsketchCalc on the given col.
func (*GreptimeDataRetriever) uddsketchCalc(col ColumnBlob) string {
	return fmt.Sprintf(uddsketchCalcFormatter, col.Name())
}
