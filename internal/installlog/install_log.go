package installlog

import (
	"context"

	octantv1 "github.com/mydecisive/octant/api/v1"
	"github.com/mydecisive/octant/internal/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
	GetInstallLog(ctx context.Context) (*octantv1.OctantInstallLogSpec, error)
	AddInstallLogEvent(ctx context.Context, entry *octantv1.OctantInstallEvent) error
}

type CustomResourceInstallLogStore struct {
	configuration *config.Configuration
	dynamicClient dynamic.Interface
}

func NewCustomResourceInstallLogStore(
	configuration *config.Configuration,
	dynamicClient dynamic.Interface,
) *CustomResourceInstallLogStore {
	return &CustomResourceInstallLogStore{
		configuration: configuration,
		dynamicClient: dynamicClient,
	}
}

func (crils *CustomResourceInstallLogStore) GetInstallLog(ctx context.Context) (*octantv1.OctantInstallLogSpec, error) {
	logger := zap.L()
	installLogResource, err := crils.loadOrCreateInstallLogResource(ctx)
	if err != nil {
		logger.Error("failed to get/create install log resource", zap.Error(err))
		return nil, err
	}
	return &installLogResource.Spec, nil
}

func (crils *CustomResourceInstallLogStore) AddInstallLogEvent(
	ctx context.Context,
	entry *octantv1.OctantInstallEvent,
) error {
	// TODO: Address behavior when number of events is high (> 1000).
	logger := zap.L()

	_, err := crils.loadOrCreateInstallLogResource(ctx)
	if err != nil {
		logger.Error("failed to get/create install log resource", zap.Error(err))
		return err
	}
	if err := crils.upsertInstallLogEntry(ctx, entry); err != nil {
		logger.Error("failed to upsert install log event", zap.Error(err))
		return err
	}
	return nil
}

func (crils *CustomResourceInstallLogStore) loadOrCreateInstallLogResource(
	ctx context.Context,
) (*octantv1.OctantInstallLog, error) {
	namespace := crils.configuration.CurrentNamespace

	var installLog *octantv1.OctantInstallLog
	rawInstallLog, err := crils.dynamicClient.Resource(
		octantv1.GetOctantInstallLogGroupVersionResource(),
	).Namespace(namespace).Get(
		ctx,
		installLogName,
		metav1.GetOptions{},
	)
	// short-circuiting on no errors instead because it's more terse in this case
	if err == nil {
		if convertErr := runtime.DefaultUnstructuredConverter.FromUnstructured(
			rawInstallLog.Object,
			&installLog,
		); convertErr != nil {
			return nil, errors.Wrap(convertErr, "failed to convert created install log back into typed object")
		}
		return installLog, nil
	}

	if !k8serrors.IsNotFound(err) {
		return nil, err
	}
	createdInstallLog, createErr := crils.createInstallLogResource(ctx)
	if createErr != nil {
		return nil, createErr
	}
	installLog = createdInstallLog

	return installLog, nil
}

func (crils *CustomResourceInstallLogStore) createInstallLogResource(
	ctx context.Context,
) (*octantv1.OctantInstallLog, error) {
	namespace := crils.configuration.CurrentNamespace
	installLogResource := &octantv1.OctantInstallLog{
		TypeMeta: metav1.TypeMeta{
			APIVersion: octantv1.GetOctantInstallLogAPIVersion(),
			Kind:       octantv1.GetOctantInstallLogKind(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      installLogName,
			Namespace: namespace,
		},
		Spec: octantv1.OctantInstallLogSpec{
			Events: make([]octantv1.OctantInstallEvent, 0),
		},
	}

	createOpts := metav1.CreateOptions{}
	unstructuredRes, err := runtime.DefaultUnstructuredConverter.ToUnstructured(installLogResource)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"failed to convert OctantInstallLog instance to unstructured type for k8s dynamic client",
		)
	}
	rawCreatedInstallLog, err := crils.dynamicClient.Resource(
		octantv1.GetOctantInstallLogGroupVersionResource(),
	).Namespace(namespace).Create(
		ctx,
		&unstructured.Unstructured{Object: unstructuredRes},
		createOpts,
	)
	if err != nil {
		return nil, err
	}
	var createdInstallLog octantv1.OctantInstallLog
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		rawCreatedInstallLog.Object,
		&createdInstallLog,
	); err != nil {
		return nil, errors.Wrap(err, "failed to convert created install log back into typed object")
	}
	return &createdInstallLog, nil
}

type patchOp string

const addOp patchOp = "add"

type patchPath string

const endOfSpecEventsPath patchPath = "/spec/events/-"

type eventPatchPayloadOperation struct {
	Op    patchOp                     `json:"op"`   // always add
	Path  patchPath                   `json:"path"` // always
	Value octantv1.OctantInstallEvent `json:"value"`
}

func (crils *CustomResourceInstallLogStore) upsertInstallLogEntry(
	ctx context.Context,
	event *octantv1.OctantInstallEvent,
) error {
	namespace := crils.configuration.CurrentNamespace

	patchPayload := []eventPatchPayloadOperation{
		{
			Op:    addOp,
			Path:  endOfSpecEventsPath,
			Value: *event,
		},
	}
	patchJSON, err := json.Marshal(patchPayload)
	if err != nil {
		return errors.Wrap(err, "failed to marshal patch payload")
	}

	if _, err := crils.dynamicClient.Resource(
		octantv1.GetOctantInstallLogGroupVersionResource(),
	).Namespace(namespace).Patch(
		ctx,
		installLogName,
		types.JSONPatchType,
		patchJSON,
		metav1.PatchOptions{},
	); err != nil {
		return err
	}

	return nil
}
