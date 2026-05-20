package budgetfilter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/mydecisive/octant/internal/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// SettingController provide abilities to get and set the budget filter settings.
type SettingController interface {
	// GetFilter returns filter setting of the given type.
	// If an update is in progress for the given type, this will return `ErrStillUpdating`.
	GetFilter(filterType budgetv1alpha.FilterType, namespace string, connection string) (*budgetv1alpha.Filter, error)
	// UpdateFilter updates the filter setting of the given type with the provided values.
	// If an update is already in progress for the given type, this will return `ErrStillUpdating`.
	// If the update takes longer than the timeout, this will return `ErrTimeout`.
	UpdateFilter(
		ctx context.Context,
		namespace string,
		connection string,
		updates *budgetv1alpha.Filter,
		out chan UpdateFilterResult,
	)
	// InitializeFilter initializes both log filter setting and
	// trace filter setting with the default values from configuration.
	// If an update is already in progress for the given type, this will return `ErrStillUpdating`.
	// If the update takes longer than the timeout, this will return `ErrTimeout`.
	InitializeFilter(
		ctx context.Context,
		namespace string,
		connection string,
		out chan UpdateFilterResult,
	)
}

// UpdateFilterResult represents results UpdateFilter method can send.
type UpdateFilterResult struct {
	Type   budgetv1alpha.FilterType
	Status budgetv1alpha.UpdateFilterResponse_Status
	Err    error
}

var (
	ErrStillUpdating   = errors.New("still updating")
	ErrInvalid         = errors.New("invalid")
	ErrFormat          = errors.New("format")
	ErrNotFound        = errors.New("not found")
	ErrTimeout         = errors.New("timeout")
	ErrUpdateValue     = errors.New("update values")
	ErrUpdateCollector = errors.New("update collectors")
)

const (
	// collectorLogNameFormatter used to generate the name of the OTEL collector
	// that is responsible for applying the log filters.
	// Used by the update process to wait for the update to complete.
	collectorLogNameFormatter = "%s-log-sampling-collector"
	// collectorTraceNameFormatter used to generate the name of the OTEL collector
	// that is responsible for applying the trace filters.
	// Used by the update process to wait for the update to complete.
	collectorTraceNameFormatter = "%s-trace-sampling-collector"
	// varLogsRatioNumber contains the hub manual variable name that corresponds to log PctSampled.
	varLogsRatioNumber = "logs_ratio_number"
	// varLogsPersistErrors contains the hub manual variable name that corresponds to log IncludeErr.
	varLogsPersistErrors = "logs_persist_errors"
	// varTracesRatioNumber contains the hub manual variable name that corresponds to trace PctSampled.
	varTracesRatioNumber = "traces_ratio_number"
	// varTracesPersistErrors contains the hub manual variable name that corresponds to trace IncludeErr.
	varTracesPersistErrors = "traces_persist_errors"
)

// settingInput used by the unexported methods to pass values.
type settingInput struct {
	namespace    string
	connection   string
	ratioVarName string
	errorVarName string
}

// MDAISettingController implements SettingController.
type MDAISettingController struct {
	log           *sync.RWMutex
	trace         *sync.RWMutex
	accessor      budgetdata.VariableAccessor
	kube          kubernetes.Interface
	configuration *config.Configuration
}

// Ensure MDAISettingController implements SettingController.
var _ SettingController = &MDAISettingController{}

// NewMDAISettingController returns a new instance of MDAISettingController.
func NewMDAISettingController(
	configuration *config.Configuration,
	accessor budgetdata.VariableAccessor,
	kube kubernetes.Interface,
) *MDAISettingController {
	return &MDAISettingController{
		log:           new(sync.RWMutex),
		trace:         new(sync.RWMutex),
		accessor:      accessor,
		kube:          kube,
		configuration: configuration,
	}
}

// GetFilter returns filter setting of the given type.
// If an update is in progress for the given type, this will return `ErrStillUpdating`.
func (sc *MDAISettingController) GetFilter(
	filterType budgetv1alpha.FilterType,
	namespace string,
	connection string,
) (*budgetv1alpha.Filter, error) {
	input := settingInput{
		namespace:  namespace,
		connection: connection,
	}

	switch filterType {
	case budgetv1alpha.FilterType_FILTER_TYPE_LOG:
		input.ratioVarName = varLogsRatioNumber
		input.errorVarName = varLogsPersistErrors
		if !sc.log.TryRLock() {
			return nil, ErrStillUpdating
		}
		defer sc.log.RUnlock()
	case budgetv1alpha.FilterType_FILTER_TYPE_TRACE:
		input.ratioVarName = varTracesRatioNumber
		input.errorVarName = varTracesPersistErrors
		if !sc.trace.TryRLock() {
			return nil, ErrStillUpdating
		}
		defer sc.trace.RUnlock()
	default:
		return nil, ErrInvalid
	}
	return sc.get(input)
}

// InitializeFilter initializes both log filter setting and
// trace filter setting with the default values from configuration.
// If an update is already in progress for the given type, this will return `ErrStillUpdating`.
// If the update takes longer than the timeout, this will return `ErrTimeout`.
func (sc *MDAISettingController) InitializeFilter(
	ctx context.Context,
	namespace string,
	connection string,
	out chan UpdateFilterResult,
) {
	outs := []chan UpdateFilterResult{
		make(chan UpdateFilterResult),
		make(chan UpdateFilterResult),
	}

	wg := sync.WaitGroup{}
	wg.Add(len(outs))

	go sc.UpdateFilter(ctx, namespace, connection, &budgetv1alpha.Filter{
		Type:       budgetv1alpha.FilterType_FILTER_TYPE_LOG,
		PctSampled: sc.configuration.Budget.DefaultLogSamplingRatio,
		IncludeErr: sc.configuration.Budget.DefaultLogIncludeErr,
	}, outs[0])

	go sc.UpdateFilter(ctx, namespace, connection, &budgetv1alpha.Filter{
		Type:       budgetv1alpha.FilterType_FILTER_TYPE_TRACE,
		PctSampled: sc.configuration.Budget.DefaultTraceSamplingRatio,
		IncludeErr: sc.configuration.Budget.DefaultTraceIncludeErr,
	}, outs[1])

	for _, ch := range outs {
		go func(ch chan UpdateFilterResult) {
			for o := range ch {
				select {
				case <-ctx.Done():
					wg.Done()
					return
				default:
				}
				out <- o
			}
			wg.Done()
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
}

// UpdateFilter updates the filter setting of the given type with the provided values.
// If an update is already in progress for the given type, this will return `ErrStillUpdating`.
// If the update takes longer than the timeout, this will return `ErrTimeout`.
func (sc *MDAISettingController) UpdateFilter(
	ctx context.Context,
	namespace string,
	connection string,
	updates *budgetv1alpha.Filter,
	out chan UpdateFilterResult,
) {
	defer close(out) // Tell caller the operation is complete
	var collectorFormatter string
	input := settingInput{
		namespace:  namespace,
		connection: connection,
	}

	switch updates.GetType() {
	case budgetv1alpha.FilterType_FILTER_TYPE_LOG:
		input.ratioVarName = varLogsRatioNumber
		input.errorVarName = varLogsPersistErrors
		collectorFormatter = collectorLogNameFormatter

		if !sc.log.TryLock() {
			out <- UpdateFilterResult{
				Type: updates.GetType(),
				Err:  ErrStillUpdating,
			}
			return
		}
		defer sc.log.Unlock()
	case budgetv1alpha.FilterType_FILTER_TYPE_TRACE:
		input.ratioVarName = varTracesRatioNumber
		input.errorVarName = varTracesPersistErrors
		collectorFormatter = collectorTraceNameFormatter

		if !sc.trace.TryLock() {
			out <- UpdateFilterResult{
				Type: updates.GetType(),
				Err:  ErrStillUpdating,
			}
			return
		}
		defer sc.trace.Unlock()
	default:
		out <- UpdateFilterResult{
			Type: updates.GetType(),
			Err:  ErrInvalid,
		}
		return
	}

	sc.update(ctx, input, collectorFormatter, updates, out)
}

// getFilter retrieves the filter settings from MDAI gateway and parse them into `budgetv1alpha.Filter`.
func (sc *MDAISettingController) get(input settingInput) (*budgetv1alpha.Filter, error) {
	filter := &budgetv1alpha.Filter{}

	numStr, err := sc.accessor.GetVariable(input.namespace, input.connection, input.ratioVarName)
	if err != nil {
		return nil, fmt.Errorf("%s:%w:%w", input.ratioVarName, ErrNotFound, err)
	}

	if numStr != "" {
		num, errNum := strconv.ParseUint(numStr, 10, 32)
		if errNum != nil {
			return nil, fmt.Errorf("%s:%w:%w", input.ratioVarName, ErrFormat, errNum)
		}
		filter.PctSampled = uint32(num)
	}

	boolStr, err := sc.accessor.GetVariable(input.namespace, input.connection, input.errorVarName)
	if err != nil {
		return nil, fmt.Errorf("%s:%w:%w", input.errorVarName, ErrNotFound, err)
	}

	if boolStr != "" {
		persistErr, errBool := strconv.ParseBool(boolStr)
		if errBool != nil {
			return nil, fmt.Errorf("%s:%w:%w", input.errorVarName, ErrFormat, errBool)
		}
		filter.IncludeErr = persistErr
	}

	return filter, nil
}

// update updates the filter settings in MDAI gateway
// and then wait for the changes to propagate to the collector pod(s).
func (sc *MDAISettingController) update(
	ctx context.Context,
	input settingInput,
	collectorNameFormatter string,
	updates *budgetv1alpha.Filter,
	out chan UpdateFilterResult,
) {
	if err := sc.accessor.UpdateVariable(
		input.namespace,
		input.connection,
		input.ratioVarName,
		strconv.FormatUint(uint64(updates.GetPctSampled()), 10),
	); err != nil {
		out <- UpdateFilterResult{
			Type: updates.GetType(),
			Err:  fmt.Errorf("%w:%w", ErrUpdateValue, err),
		}
		return
	}
	if err := sc.accessor.UpdateVariable(
		input.namespace,
		input.connection,
		input.errorVarName,
		updates.GetIncludeErr(),
	); err != nil {
		out <- UpdateFilterResult{
			Type: updates.GetType(),
			Err:  fmt.Errorf("%w:%w", ErrUpdateValue, err),
		}
		return
	}

	out <- UpdateFilterResult{
		Type:   updates.GetType(),
		Status: budgetv1alpha.UpdateFilterResponse_STATUS_VALUE_UPDATED,
	}

	if err := wait.PollUntilContextTimeout(ctx,
		time.Duration(sc.configuration.Budget.FilterSettingUpdateInterval)*time.Second,
		time.Duration(sc.configuration.Budget.FilterSettingUpdateTimeout)*time.Second,
		true,
		func(ctx context.Context) (bool, error) {
			out <- UpdateFilterResult{
				Type:   updates.GetType(),
				Status: budgetv1alpha.UpdateFilterResponse_STATUS_WAIT_PROPAGATION,
			}
			deployment, err := sc.kube.AppsV1().
				Deployments(input.namespace).
				Get(ctx, fmt.Sprintf(collectorNameFormatter, input.connection), metav1.GetOptions{})
			if err != nil {
				return true, err
			}
			// nolint: gocritic // no this cond is not sus
			return deployment.Status.Replicas == deployment.Status.ReadyReplicas &&
				deployment.Status.Replicas == deployment.Status.UpdatedReplicas, nil
		}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			out <- UpdateFilterResult{
				Type: updates.GetType(),
				Err:  fmt.Errorf("%w:%w", ErrUpdateCollector, ErrTimeout),
			}
			return
		}
		out <- UpdateFilterResult{
			Type: updates.GetType(),
			Err:  fmt.Errorf("%w:%w", ErrUpdateCollector, err),
		}
		return
	}
	out <- UpdateFilterResult{
		Type:   updates.GetType(),
		Status: budgetv1alpha.UpdateFilterResponse_STATUS_COMPLETED,
	}
}
