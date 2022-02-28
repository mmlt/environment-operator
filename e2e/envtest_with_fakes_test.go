package e2e

import (
	"context"
	"encoding/base64"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/testr"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
	"time"
)

func TestGoodRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	logf.SetLogger(testr.New(t))

	wg := testManagerWithFakeClients(t, ctx, testLabels)
	tf := testReconciler.Planner.Terraform.(*terraform.TerraformFake)
	kc := testReconciler.Planner.Kubectl.(*kubectl.KubectlFake)

	t.Run("should_run_all_steps_to_create_or_update_a_cluster", func(t *testing.T) {
		tf.SetupFakeResultsForCreate(map[string]interface{}{
			"xyz": map[string]interface{}{
				"kube_admin_config": map[string]interface{}{
					"client_certificate":     base64.StdEncoding.EncodeToString(cfg.CertData),
					"client_key":             base64.StdEncoding.EncodeToString(cfg.KeyData),
					"cluster_ca_certificate": base64.StdEncoding.EncodeToString(cfg.CAData),
					"host":                   cfg.Host,
					"password":               cfg.Password,
					"username":               cfg.Username,
				},
			},
		})
		testCreateCR(t, testEnvironmentCR(testNSN, testLabels, testSpecLocal(1)))

		got := testGetCRWhenConditionReady(t, testNSN)

		/*TODO		// Metrics
		cnt := testutil.ToFloat64(executor.MetricSteps)
		assert.EqualValues(t, float64(7), cnt, "number of executed steps")
		cnt = testutil.ToFloat64(executor.MetricStepFailures)
		assert.EqualValues(t, float64(0), cnt, "number of failed executed steps")
		*/
		// Condition
		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, v1.ReasonReady, got.Status.Conditions[0].Reason)

		// Steps
		assert.Equal(t, 4, len(got.Status.Steps), "number of Status.Steps")

		assert.Equal(t, v1.StateReady, got.Status.Steps["Infra"].State, "Status.Steps[Infra].State")
		assert.Equal(t, "terraform apply errors=0 added=1 changed=2 deleted=1", got.Status.Steps["Infra"].Message)

		assert.Equal(t, v1.StateReady, got.Status.Steps["AKSPoolxyz"].State, "Status.Steps[AKSPoolxyz].State")

		assert.Equal(t, v1.StateReady, got.Status.Steps["AKSAddonPreflightxyz"].State, "Status.Steps[AKSAddonPreflightxyz].State")

		assert.Equal(t, v1.StateReady, got.Status.Steps["Addonsxyz"].State, "Status.Steps[Addonsxyz].State")
		assert.Equal(t, "kubectl-tmplt errors=0 added=0 changed=1 deleted=0", got.Status.Steps["Addonsxyz"].Message)

		// Check cluster Secret creation.
		ss := testListSecrets(t)
		if assert.Equal(t, 1, len(ss)) {
			// label propagation
			assert.Equal(t, map[string]string(testLabels), ss[0].Labels)
		}
	})

	t.Run("should_be_able_to_remove_a_cluster", func(t *testing.T) {
		tf.SetupFakeResultsForDeleteCluster()
		testCreateCR(t, testEnvironmentCR(testNSN, testLabels, testSpecLocal(0)))

		got := testGetCRWhenConditionReady(t, testNSN)

		// Condition
		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, v1.ReasonReady, got.Status.Conditions[0].Reason)

		assert.Equal(t, 1, kc.WipeClusterTally)
	})

	t.Run("should_handle_nothing_to_do", func(t *testing.T) {
		tf.SetupFakeResultsForNothingToDo()
		testCreateCR(t, testEnvironmentCR(testNSN, testLabels, testSpecLocal(1)))

		got := testGetCRWhenConditionReady(t, testNSN)

		// Condition
		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, v1.ReasonReady, got.Status.Conditions[0].Reason)

		assert.Equal(t, 4, len(got.Status.Steps), "expected 4 Status.Steps of state Ready")
	})

	// teardown manager
	cancel()
	wg.Wait()
}

func TestErrorRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	logf.SetLogger(testr.New(t))

	wg := testManagerWithFakeClients(t, ctx, testLabels)
	tf := testReconciler.Planner.Terraform.(*terraform.TerraformFake)

	t.Run("should_be_able_to_reset_step", func(t *testing.T) {
		// Run step that will fail.
		tf.SetupFakeResultsForFailedDestroy()
		testCreateCR(t, testEnvironmentCR(testNSN, testLabels, testSpecLocalDestroy()))

		got := testGetCRWhenConditionReady(t, testNSN)

		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, metav1.ConditionTrue, got.Status.Conditions[0].Status)
		assert.Equal(t, v1.ReasonFailed, got.Status.Conditions[0].Reason)
		assert.Equal(t, "0/1 ready, 0 running, 1 error(s)", got.Status.Conditions[0].Message)

		assert.Equal(t, 1, len(got.Status.Steps), "number of Status.Steps")
		assert.Equal(t, v1.StateError, got.Status.Steps["Destroy"].State, "Status.Steps[Destroy].State")
		assert.Equal(t, "did not receive response from terraform destroy", got.Status.Steps["Destroy"].Message)

		// Fix error and reset step.
		tf.SetupFakeResultsForSuccessfulDestroy()
		testResetStep(t, testNSN, "Destroy")
		time.Sleep(5 * time.Second) // it will take some time for the condition to reflect the new status (should ResetStep also remove Condition?)

		got = testGetCRWhenConditionReady(t, testNSN)

		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, metav1.ConditionTrue, got.Status.Conditions[0].Status)
		assert.Equal(t, v1.ReasonReady, got.Status.Conditions[0].Reason)
		assert.Equal(t, "1/1 ready, 0 running, 0 error(s)", got.Status.Conditions[0].Message)

		assert.Equal(t, 1, len(got.Status.Steps), "number of Status.Steps")
		assert.Equal(t, v1.StateReady, got.Status.Steps["Destroy"].State, "Status.Steps[Destroy].State")
		assert.Equal(t, "terraform destroy errors=0 added=0 changed=0 deleted=1", got.Status.Steps["Destroy"].Message)
	})

	// teardown manager
	cancel()
	wg.Wait()
}
