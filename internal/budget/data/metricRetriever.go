package budgetdata

import (
	"context"
	"database/sql"
	"fmt"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	. "github.com/go-jet/jet/v2/mysql" //nolint
	budgetdb "github.com/mydecisive/octant/internal/budget/data/db"
	. "github.com/mydecisive/octant/internal/budget/data/db/public/table" //nolint
	"github.com/mydecisive/octant/internal/config"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

const (
	toGB        = 1073741824.0
	toMilEvents = 1000000.0

	uddsketchCalcFormatter = "uddsketch_calc(0.50, uddsketch_merge(128, 0.01, %s))"
)

const (
	dayInHR       = 24   // 1 day
	monthInHR     = 730  // 30 days (i.e., closest approx. to a month)
	lastMonthInHR = 1460 // 60 days (i.e., closest approx. to 2 month)
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
}

// Ensure GreptimeDataRetriever implements MetricDataRetriever.
var _ MetricDataRetriever = &GreptimeDataRetriever{}

type GreptimeDataRetriever struct {
	config    *config.Configuration
	k8sClient kubernetes.Interface
	builder   budgetdb.DatabaseAccessBuilder
}

// NewGreptimeDataRetriever creates a new instance of GreptimeDataRetriever.
func NewGreptimeDataRetriever(
	con *config.Configuration,
	k8sClient kubernetes.Interface,
	builder budgetdb.DatabaseAccessBuilder,
) *GreptimeDataRetriever {
	return &GreptimeDataRetriever{
		config:    con,
		k8sClient: k8sClient,
		builder:   builder,
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
		return nil, err
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
		return nil, fmt.Errorf("log received:%w", err)
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
		return nil, fmt.Errorf("log sent:%w", err)
	}

	spanRec, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		ReceivedSpanRootCountTotal,
		ReceivedSpanRootCountTotal.GreptimeValue,
		ReceivedSpanRootCountTotal.GreptimeTimestamp,
		toMilEvents,
	)
	if err != nil {
		return nil, fmt.Errorf("span received:%w", err)
	}

	spanSent, err := gdr.getTotal(
		ctx,
		conn.DB,
		timeframe,
		SentSpanCountTotal,
		SentSpanCountTotal.GreptimeValue,
		SentSpanCountTotal.GreptimeTimestamp,
		toMilEvents,
	)
	if err != nil {
		return nil, fmt.Errorf("span sent:%w", err)
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
		return -1, err
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
		return nil, "", err
	}

	where := gdr.timeRangeExpression(input.Timeframe, table.GreptimeTimestamp)
	if input.Search != "" {
		where.AND(table.Service.LIKE(String("%" + input.Search + "%")))
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
		return nil, "", err
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
		return nil, "", err
	}

	where := gdr.timeRangeExpression(input.Timeframe, table.TimeWindow)
	if input.Search != "" {
		where.AND(table.RootID.LIKE(String("%" + input.Search + "%")))
	}
	stmt := SELECT(
		table.RootID.AS("root_span.name"),
		SUM(CAST(table.TraceCount).AS_FLOAT().DIV(Float(toMilEvents))).AS("root_span.count"),
		RawFloat(fmt.Sprintf(uddsketchCalcFormatter, table.BreadthSketch.Name())).AS("root_span.breadth"),
		RawFloat(fmt.Sprintf(uddsketchCalcFormatter, table.DepthSketch.Name())).AS("root_span.depth"),
		RawFloat(fmt.Sprintf(uddsketchCalcFormatter, table.DurationSketch.Name())).AS("root_span.invocation"),
	).FROM(table).
		WHERE(where).
		GROUP_BY(table.RootID).
		ORDER_BY(Raw("`root_span.count` DESC")).
		LIMIT(int64(input.Size + 1))

	zap.L().Info(stmt.DebugSql())
	var result []RootSpan
	if err := stmt.QueryContext(ctx, conn.DB, &result); err != nil {
		return nil, "", err
	}

	next := ""
	if len(result) > int(input.Size) {
		next = result[int(input.Size)].Name
		result = result[:len(result)-1]
	}
	return result, next, nil
}

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
		return -1, err
	}
	if len(result) > 0 {
		return result[0], nil
	}
	return -1, budgetdb.ErrMissing
}

func (gdr *GreptimeDataRetriever) timeRangeExpression( //nolint:ireturn
	timeframe budgetv1alpha.Timeframe,
	timestampCol ColumnString,
) BoolExpression {
	return CAST(timestampCol).AS_TIME().
		GT_EQ(
			CAST(NOW().SUB(INTERVAL(gdr.toHr(timeframe), HOUR))).AS_TIME(),
		)
}

func (*GreptimeDataRetriever) toHr(timeframe budgetv1alpha.Timeframe) int {
	switch timeframe {
	case budgetv1alpha.Timeframe_TIMEFRAME_24HR:
		return dayInHR
	case budgetv1alpha.Timeframe_TIMEFRAME_MTD:
		return monthInHR
	default:
		return lastMonthInHR
	}
}
