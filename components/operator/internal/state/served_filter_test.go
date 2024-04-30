package state

import (
	"context"
	"testing"

	"github.com/kyma-project/docker-registry/components/operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_sFnServedFilter(t *testing.T) {
	t.Run("skip processing when served is false", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.DockerRegistry{
				Status: v1alpha1.DockerRegistryStatus{
					Served: v1alpha1.ServedFalse,
				},
			},
		}

		nextFn, result, err := sFnServedFilter(context.TODO(), nil, s)
		require.Nil(t, err)
		require.Nil(t, result)
		require.Nil(t, nextFn)
	})

	t.Run("do next step when served is true", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.DockerRegistry{
				Status: v1alpha1.DockerRegistryStatus{
					Served: v1alpha1.ServedTrue,
				},
			},
		}

		nextFn, result, err := sFnServedFilter(context.TODO(), nil, s)
		require.Nil(t, err)
		require.Nil(t, result)
		requireEqualFunc(t, sFnAddFinalizer, nextFn)
	})

	t.Run("set served value from nil to true when there is no served dockerregistry on cluster", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.DockerRegistry{
				Status: v1alpha1.DockerRegistryStatus{},
			},
		}

		r := &reconciler{
			k8s: k8s{
				client: func() client.Client {
					scheme := apiruntime.NewScheme()
					require.NoError(t, v1alpha1.AddToScheme(scheme))

					client := fake.NewClientBuilder().
						WithScheme(scheme).
						WithObjects(
							fixServedDockerRegistry("test-1", "default", ""),
							fixServedDockerRegistry("test-2", "dockerregistry-test", v1alpha1.ServedFalse),
							fixServedDockerRegistry("test-3", "dockerregistry-test-2", ""),
							fixServedDockerRegistry("test-4", "default", v1alpha1.ServedFalse),
						).Build()

					return client
				}(),
			},
		}

		nextFn, result, err := sFnServedFilter(context.TODO(), r, s)
		require.Nil(t, err)
		require.Nil(t, result)
		requireEqualFunc(t, sFnAddFinalizer, nextFn)
		require.Equal(t, v1alpha1.ServedTrue, s.instance.Status.Served)
	})

	t.Run("set served value from nil to false and set condition to error when there is at lease one served dockerregistry on cluster", func(t *testing.T) {
		s := &systemState{
			instance: v1alpha1.DockerRegistry{
				Status: v1alpha1.DockerRegistryStatus{},
			},
		}

		r := &reconciler{
			k8s: k8s{
				client: func() client.Client {
					scheme := apiruntime.NewScheme()
					require.NoError(t, v1alpha1.AddToScheme(scheme))

					client := fake.NewClientBuilder().
						WithScheme(scheme).
						WithObjects(
							fixServedDockerRegistry("test-1", "default", v1alpha1.ServedFalse),
							fixServedDockerRegistry("test-2", "dockerregistry-test", v1alpha1.ServedTrue),
							fixServedDockerRegistry("test-3", "dockerregistry-test-2", ""),
							fixServedDockerRegistry("test-4", "default", v1alpha1.ServedFalse),
						).Build()

					return client
				}(),
			},
		}

		nextFn, result, err := sFnServedFilter(context.TODO(), r, s)

		expectedErrorMessage := "Only one instance of DockerRegistry is allowed (current served instance: dockerregistry-test/test-2). This DockerRegistry CR is redundant. Remove it to fix the problem."
		require.EqualError(t, err, expectedErrorMessage)
		require.Nil(t, result)
		require.Nil(t, nextFn)
		require.Equal(t, v1alpha1.ServedFalse, s.instance.Status.Served)

		status := s.instance.Status
		require.Equal(t, v1alpha1.StateWarning, status.State)
		requireContainsCondition(t, status,
			v1alpha1.ConditionTypeConfigured,
			metav1.ConditionFalse,
			v1alpha1.ConditionReasonDuplicated,
			expectedErrorMessage,
		)
	})
}

func fixServedDockerRegistry(name, namespace string, served v1alpha1.Served) *v1alpha1.DockerRegistry {
	return &v1alpha1.DockerRegistry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: v1alpha1.DockerRegistryStatus{
			Served: served,
		},
	}
}
