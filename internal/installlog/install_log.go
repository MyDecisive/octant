package installlog

import (
	"context"
	v1 "github.com/mydecisive/octant/api/v1"
	"github.com/mydecisive/octant/internal/config"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
)

const (
	installLogName = "octant-install-log" // we use a single, canonical install log CR
)

type InstallLogStore interface {
	GetInstallLog(ctx context.Context) (*v1.OctantInstallLogSpec, error)
	AddInstallLogEvent(ctx context.Context, entry *v1.OctantInstallEvent) error
}

type CustomResourceInstallLogStore struct {
	configuration config.Configuration
	dynamicClient dynamic.Interface
}

func NewCustomResourceInstallLogStore(configuration config.Configuration, dynamicClient dynamic.Interface) *CustomResourceInstallLogStore {
	return &CustomResourceInstallLogStore{
		configuration: configuration,
		dynamicClient: dynamicClient,
	}
}

func (crils *CustomResourceInstallLogStore) GetInstallLog(ctx context.Context) (*v1.OctantInstallLogSpec, error) {
	installLogResource, err := crils.loadInstallLogResource(ctx)
	if err != nil {
		zap.Error(err)
		return nil, err
	}
	return &installLogResource.Spec, nil
}

// TODO: Address behavior when number of events is high (> 1000)
func (crils *CustomResourceInstallLogStore) AddInstallLogEvent(ctx context.Context, entry *v1.OctantInstallEvent) error {
	installLogResource, err := crils.loadInstallLogResource(ctx)
	if err != nil {
		zap.Error(err)
		return err
	}
	if installLogResource == nil {
		createdInstallLog, createErr := crils.createInstallLogResource(ctx)
		if createErr != nil {
			zap.Error(createErr)
			return createErr
		}
		installLogResource = createdInstallLog
	}
	if err := crils.upsertInstallLogEntry(ctx, entry); err != nil {
		zap.Error(err)
		return err
	}
	return nil
}

func (crils *CustomResourceInstallLogStore) loadInstallLogResource(ctx context.Context) (*v1.OctantInstallLog, error) {
	logger := zap.L()
	namespace := crils.configuration.CurrentNamespace

	rawInstallLog, err := crils.dynamicClient.Resource(v1.GetOctantInstallLogGroupVersionResource()).Namespace(namespace).Get(ctx, installLogName, metav1.GetOptions{})
	if err != nil {
		logger.Error("Failed to get install log", zap.Error(err))
		return nil, err
	}
	var installLog v1.OctantInstallLog
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawInstallLog.Object, &installLog); err != nil {
		logger.Error("WEIRD: failed to convert created install log back into typed object.", zap.Error(err))
	}

	return &installLog, nil
}

func (crils *CustomResourceInstallLogStore) createInstallLogResource(ctx context.Context) (*v1.OctantInstallLog, error) {
	logger := zap.L()
	namespace := crils.configuration.CurrentNamespace
	installLogResource := &v1.OctantInstallLog{
		ObjectMeta: metav1.ObjectMeta{
			Name:      installLogName,
			Namespace: namespace,
		},
		Spec: v1.OctantInstallLogSpec{
			Events: make([]v1.OctantInstallEvent, 0),
		},
	}

	createOpts := metav1.CreateOptions{}
	unstructuredRes, err := runtime.DefaultUnstructuredConverter.ToUnstructured(installLogResource)
	if err != nil {
		logger.Error("WEIRD: failed to convert OctantInstallLog instance to unstructured type for k8s dynamic client", zap.Error(err))
		return nil, err
	}
	rawCreatedInstallLog, err := crils.dynamicClient.Resource(v1.GetOctantInstallLogGroupVersionResource()).Create(ctx, &unstructured.Unstructured{Object: unstructuredRes}, createOpts)
	if err != nil {
		logger.Error("error occurred while creating OctantInstallLog custom resource", zap.Error(err))
		return nil, err
	}
	var createdInstallLog v1.OctantInstallLog
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(rawCreatedInstallLog.Object, &createdInstallLog); err != nil {
		logger.Error("WEIRD: failed to convert created install log back into typed object.", zap.Error(err))
	}
	return &createdInstallLog, nil
}

type patchOp string

const addOp patchOp = "add"

type patchPath string

const endOfSpecEventsPath patchPath = "/spec/events/-"

type eventPatchPayloadOperation struct {
	Op    patchOp               `json:"op"`   // always add
	Path  patchPath             `json:"path"` // always
	Value v1.OctantInstallEvent `json:"value,omitempty"`
}

func (crils *CustomResourceInstallLogStore) upsertInstallLogEntry(ctx context.Context, event *v1.OctantInstallEvent) error {
	logger := zap.L()

	patchPayload := []eventPatchPayloadOperation{
		{
			Op:    addOp,
			Path:  endOfSpecEventsPath,
			Value: *event,
		},
	}
	patchJson, err := json.Marshal(patchPayload)
	if err != nil {
		logger.Error("WEIRD: failed to marshal patch payload", zap.Error(err))
		return err
	}

	if _, err := crils.dynamicClient.Resource(v1.GetOctantInstallLogGroupVersionResource()).Patch(ctx, installLogName, types.JSONPatchType, patchJson, metav1.PatchOptions{}); err != nil {
		logger.Error("error occurred while patching install log entry", zap.Error(err))
		return err
	}

	return nil
}
