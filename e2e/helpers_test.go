package e2e

import (
	"fmt"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"
)

func testCreateCR(t *testing.T, cr *v1.Environment) {
	t.Helper()

	nsn := types.NamespacedName{
		Namespace: cr.Namespace,
		Name:      cr.Name,
	}
	testDeleteCR(t, nsn)
	err := k8sClient.Create(testCtx, cr)
	assert.NoError(t, err)
}

func testDeleteCR(t *testing.T, nsn types.NamespacedName) {
	t.Helper()

	obj := &v1.Environment{}
	err := k8sClient.Get(testCtx, nsn, obj)
	if apierrors.IsNotFound(err) {
		return
	}
	assert.NoError(t, err)
	err = k8sClient.Delete(testCtx, obj)
	assert.NoError(t, err)
}

func testGetCR(t *testing.T, nsn types.NamespacedName) *v1.Environment {
	t.Helper()

	obj := &v1.Environment{}
	err := k8sClient.Get(testCtx, nsn, obj)
	assert.NoError(t, err)
	return obj
}

func testGetCRWhenConditionReady(t *testing.T, nsn types.NamespacedName) *v1.Environment {
	t.Helper()

	obj := &v1.Environment{}
	err := wait.Poll(time.Second, 10*time.Minute, func() (done bool, err error) {
		err = k8sClient.Get(testCtx, nsn, obj)
		if err != nil {
			return false, err
		}
		for _, c := range obj.Status.Conditions {
			if c.Type == "Ready" {
				return c.Status == metav1.ConditionTrue, nil
			}
		}
		return false, nil
	})
	assert.NoError(t, err)
	return obj
}

func testResetStep(t *testing.T, nsn types.NamespacedName, step string) {
	t.Helper()

	cr := testGetCR(t, nsn)
	p := fmt.Sprintf(`[{"op": "remove", "path": "/status/steps/%s"}]`, step)
	err := k8sClient.Status().Patch(testCtx, cr, client.RawPatch(types.JSONPatchType, []byte(p)))
	assert.NoError(t, err)
}

func testListSecrets(t *testing.T) []corev1.Secret {
	t.Helper()

	obj := &corev1.SecretList{}
	err := k8sClient.List(testCtx, obj)
	assert.NoError(t, err)

	return obj.Items
}
