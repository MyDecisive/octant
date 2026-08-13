package installlog

import (
	"testing"

	octantv1 "github.com/mydecisive/octant/api/v1"
	"github.com/mydecisive/octant/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	testNamespace = "foobaz"
)

func makeCRGivenEntries(entries []octantv1.OctantInstallEvent) *octantv1.OctantInstallLog {
	return &octantv1.OctantInstallLog{
		TypeMeta: metav1.TypeMeta{
			APIVersion: octantv1.GetOctantInstallLogAPIVersion(),
			Kind:       octantv1.GetOctantInstallLogKind(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      installLogName,
			Namespace: testNamespace,
		},
		Spec: octantv1.OctantInstallLogSpec{
			Events: entries,
		},
	}
}

func TestCustomResourceInstallLogStore_GetInstallLog(t *testing.T) {
	t.Parallel()

	t.Run("cr exists and is empty", func(t *testing.T) {
		t.Parallel()
		initialDatas := makeCRGivenEntries([]octantv1.OctantInstallEvent{})

		initialDataRaw, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(initialDatas)
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
			Object: initialDataRaw,
		})
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expected := &octantv1.OctantInstallLogSpec{
			Events: make([]octantv1.OctantInstallEvent, 0),
		}
		actual, err := logStore.GetInstallLog(t.Context())

		require.NoError(t, err)
		require.NotNil(t, actual)
		assert.Equal(t, expected, actual)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 1)
		assert.Equal(t, "get", actions[0].GetVerb())
	})

	t.Run("cr exists and has entries", func(t *testing.T) {
		t.Parallel()
		makeEntries := func() []octantv1.OctantInstallEvent {
			return []octantv1.OctantInstallEvent{
				{
					Action:    octantv1.CreateDeployIntegration,
					Result:    octantv1.FailureOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
					Subtype:   string(octantv1.ArgoCDSubtype),
					Message:   "argo said no",
				},
				{
					Action:    octantv1.CreateDeployIntegration,
					Result:    octantv1.SuccessOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
					Subtype:   string(octantv1.ArgoCDSubtype),
				},
				{
					Action:    octantv1.InstallMDAIHub,
					Result:    octantv1.SuccessOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
				},
			}
		}
		initialDatas := makeCRGivenEntries(makeEntries())

		initialDataRaw, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(initialDatas)
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
			Object: initialDataRaw,
		})
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expected := &octantv1.OctantInstallLogSpec{
			Events: makeEntries(),
		}
		actual, err := logStore.GetInstallLog(t.Context())

		require.NoError(t, err)
		require.NotNil(t, actual)
		assert.Equal(t, expected, actual)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 1)
		assert.Equal(t, "get", actions[0].GetVerb())
	})

	t.Run("cr doesn't exist", func(t *testing.T) {
		t.Parallel()
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme())
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expected := &octantv1.OctantInstallLogSpec{
			Events: make([]octantv1.OctantInstallEvent, 0),
		}
		actual, err := logStore.GetInstallLog(t.Context())

		require.NoError(t, err)
		require.NotNil(t, actual)
		assert.Equal(t, expected, actual)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 2)
		assert.Equal(t, "get", actions[0].GetVerb())
		assert.Equal(t, "create", actions[1].GetVerb())
	})
}

func TestCustomResourceInstallLogStore_AddInstallLogEvent(t *testing.T) {
	t.Parallel()

	t.Run("cr exists and is empty", func(t *testing.T) {
		t.Parallel()
		makeEntries := func() []octantv1.OctantInstallEvent {
			return []octantv1.OctantInstallEvent{
				{
					Action:    octantv1.CreateDeployIntegration,
					Result:    octantv1.FailureOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
					Subtype:   string(octantv1.ArgoCDSubtype),
					Message:   "argo said no",
				},
				{
					Action:    octantv1.CreateDeployIntegration,
					Result:    octantv1.SuccessOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
					Subtype:   string(octantv1.ArgoCDSubtype),
				},
				{
					Action:    octantv1.InstallMDAIHub,
					Result:    octantv1.SuccessOctantInstallEventResult,
					Namespace: testNamespace,
					Ref:       "argofoo",
				},
			}
		}
		initialDatas := makeCRGivenEntries(makeEntries())

		initialDataRaw, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(initialDatas)
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
			Object: initialDataRaw,
		})
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expectedEvent := octantv1.OctantInstallEvent{
			Action:    octantv1.CreateDeployIntegration,
			Result:    octantv1.FailureOctantInstallEventResult,
			Namespace: testNamespace,
			Ref:       "argofoo",
			Subtype:   string(octantv1.ArgoCDSubtype),
			Message:   "argo said no",
		}

		err := logStore.AddInstallLogEvent(t.Context(), &expectedEvent)

		require.NoError(t, err)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 2)
		assert.Equal(t, "get", actions[0].GetVerb())
		assert.Equal(t, "patch", actions[1].GetVerb())

		expectedPatch := []eventPatchPayloadOperation{
			{
				Op:    addOp,
				Path:  endOfSpecEventsPath,
				Value: expectedEvent,
			},
		}

		patchAction := actions[1].(k8stesting.PatchAction)
		patchStr := patchAction.GetPatch()
		require.NotNil(t, patchStr)
		var actualPatch []eventPatchPayloadOperation
		require.NoError(t, json.Unmarshal(patchStr, &actualPatch))
		assert.Equal(t, expectedPatch, actualPatch)
	})

	t.Run("cr exists and has entries", func(t *testing.T) {
		t.Parallel()
		initialDatas := makeCRGivenEntries([]octantv1.OctantInstallEvent{})

		initialDataRaw, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(initialDatas)
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme(), &unstructured.Unstructured{
			Object: initialDataRaw,
		})
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expectedEvent := octantv1.OctantInstallEvent{
			Action:    octantv1.CreateDeployIntegration,
			Result:    octantv1.FailureOctantInstallEventResult,
			Namespace: testNamespace,
			Ref:       "argofoo",
			Subtype:   string(octantv1.ArgoCDSubtype),
			Message:   "argo said no",
		}

		err := logStore.AddInstallLogEvent(t.Context(), &expectedEvent)

		require.NoError(t, err)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 2)
		assert.Equal(t, "get", actions[0].GetVerb())
		assert.Equal(t, "patch", actions[1].GetVerb())

		expectedPatch := []eventPatchPayloadOperation{
			{
				Op:    addOp,
				Path:  endOfSpecEventsPath,
				Value: expectedEvent,
			},
		}

		patchAction := actions[1].(k8stesting.PatchAction)
		patchStr := patchAction.GetPatch()
		require.NotNil(t, patchStr)
		var actualPatch []eventPatchPayloadOperation
		require.NoError(t, json.Unmarshal(patchStr, &actualPatch))
		assert.Equal(t, expectedPatch, actualPatch)
	})

	t.Run("cr doesn't exist", func(t *testing.T) {
		t.Parallel()
		dynamicMock := fake.NewSimpleDynamicClient(runtime.NewScheme())
		testConfig := &config.Configuration{CurrentNamespace: testNamespace}

		logStore := NewCustomResourceInstallLogStore(testConfig, dynamicMock)

		expectedEvent := octantv1.OctantInstallEvent{
			Action:    octantv1.CreateDeployIntegration,
			Result:    octantv1.FailureOctantInstallEventResult,
			Namespace: testNamespace,
			Ref:       "argofoo",
			Subtype:   string(octantv1.ArgoCDSubtype),
			Message:   "argo said no",
		}

		err := logStore.AddInstallLogEvent(t.Context(), &expectedEvent)

		require.NoError(t, err)

		actions := dynamicMock.Actions()
		assert.Len(t, actions, 3)
		assert.Equal(t, "get", actions[0].GetVerb())
		assert.Equal(t, "create", actions[1].GetVerb())
		assert.Equal(t, "patch", actions[2].GetVerb())

		expectedPatch := []eventPatchPayloadOperation{
			{
				Op:    addOp,
				Path:  endOfSpecEventsPath,
				Value: expectedEvent,
			},
		}

		patchAction := actions[2].(k8stesting.PatchAction)
		patchStr := patchAction.GetPatch()
		require.NotNil(t, patchStr)
		var actualPatch []eventPatchPayloadOperation
		require.NoError(t, json.Unmarshal(patchStr, &actualPatch))
		assert.Equal(t, expectedPatch, actualPatch)
	})
}
