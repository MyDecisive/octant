package budgetfilter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	"github.com/mydecisive/octant/internal/config"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

var (
	ErrStillUpdating = errors.New("still updating")
	ErrInvalid       = errors.New("invalid")
	ErrNotFound      = errors.New("not found")
	ErrTimeout       = errors.New("timeout")
)

const (
	collectorLogNameFormatter   = "%s-log-sampling"
	collectorTraceNameFormatter = "%s-trace-sampling"
	varLogsRatioNumber          = "logs_ratio_number"
	varLogsPersistErrors        = "logs_persist_errors"
	varTracesRatioNumber        = "traces_ratio_number"
	varTracesPersistErrors      = "traces_persist_errors"
)

type SettingController struct {
	log           *sync.Mutex
	trace         *sync.Mutex
	accessor      VariableAccessor
	kube          kubernetes.Interface
	configuration config.Configuration
}

func NewSettingController(configuration config.Configuration, accessor VariableAccessor, kube kubernetes.Interface) *SettingController {
	return &SettingController{
		log:           new(sync.Mutex),
		trace:         new(sync.Mutex),
		accessor:      accessor,
		kube:          kube,
		configuration: configuration,
	}
}

func (sc *SettingController) GetFilter(filterType budgetv1alpha.FilterType, namespace string, connectionName string) (*budgetv1alpha.Filter, error) {
	if !sc.log.TryLock() {
		return nil, ErrStillUpdating
	}

	variables, err := sc.accessor.GetAllVariables(namespace, connectionName)
	if err != nil {
		return nil, err
	}

	switch filterType {
	case budgetv1alpha.FilterType_FILTER_TYPE_LOG:
		return sc.parseFilterSetting(variables, varLogsRatioNumber, varLogsPersistErrors)
	case budgetv1alpha.FilterType_FILTER_TYPE_TRACE:
		return sc.parseFilterSetting(variables, varTracesRatioNumber, varTracesPersistErrors)
	default:
		return nil, ErrInvalid
	}
}

func (sc *SettingController) parseFilterSetting(variables map[string]string, ratioVarName string, errorVarName string) (*budgetv1alpha.Filter, error) {
	filter := &budgetv1alpha.Filter{}
	numStr, ok := variables[ratioVarName]
	if !ok {
		return nil, fmt.Errorf("%w:%s", ErrNotFound, ratioVarName)
	}
	num, err := strconv.ParseUint(numStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w:%s", ErrInvalid, ratioVarName)
	}
	filter.PctSampled = uint32(num)

	boolStr, ok := variables[errorVarName]
	if !ok {
		return nil, fmt.Errorf("%w:%s", ErrNotFound, errorVarName)
	}
	persistErr, err := strconv.ParseBool(boolStr)
	if err != nil {
		return nil, fmt.Errorf("%w:%s", ErrInvalid, errorVarName)
	}
	filter.IncludeErr = persistErr

	return filter, nil
}

func (sc *SettingController) updateLogFilter(ctx context.Context, updates *budgetv1alpha.Filter, namespace string, connectionName string) error {
	if !sc.log.TryLock() {
		return ErrStillUpdating
	}

	sc.log.Lock()
	defer sc.log.Unlock()

	if err := sc.accessor.PostVariable(namespace, connectionName, varLogsRatioNumber, strconv.FormatUint(uint64(updates.GetPctSampled()), 10)); err != nil {
		return err
	}
	if err := sc.accessor.PostVariable(namespace, connectionName, varLogsPersistErrors, updates.GetIncludeErr()); err != nil {
		return err
	}

	if err := wait.PollUntilContextTimeout(ctx,
		time.Duration(sc.configuration.Budget.FilterSettingUpdateInterval)*time.Second,
		time.Duration(sc.configuration.Budget.FilterSettingUpdateTimeout)*time.Second,
		true,
		func(ctx context.Context) (done bool, err error) {
			sc.kube.CoreV1().Pods(namespace).Get(ctx)
			return false, nil
		}); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return err
	}
	return nil
}
