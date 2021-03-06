package controllers

import (
	"context"
	v1 "github.com/mmlt/environment-operator/api/v1"
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

	wg := testManagerWithFakeClients(t, ctx)

	t.Run("should_run_all_steps", func(t *testing.T) {
		testCreateCR(t, testEnvironmentCR(testNSN, testSpecLocal()))

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
		assert.Equal(t, 5, len(got.Status.Steps), "number of Status.Steps")

		assert.Equal(t, v1.StateReady, got.Status.Steps["Infra"].State, "Status.Steps[Infra].State")
		assert.Equal(t, "terraform apply errors=0 added=1 changed=2 deleted=1", got.Status.Steps["Infra"].Message)

		assert.Equal(t, v1.StateReady, got.Status.Steps["AKSPoolxyz"].State, "Status.Steps[AKSPoolxyz].State")

		assert.Equal(t, v1.StateReady, got.Status.Steps["Kubeconfigxyz"].State, "Status.Steps[Kubeconfigxyz].State")

		assert.Equal(t, v1.StateReady, got.Status.Steps["AKSAddonPreflightxyz"].State, "Status.Steps[AKSAddonPreflightxyz].State")

		assert.Equal(t, v1.StateReady, got.Status.Steps["Addonsxyz"].State, "Status.Steps[Addonsxyz].State")
		assert.Equal(t, "kubectl-tmplt errors=0 added=0 changed=1 deleted=0", got.Status.Steps["Addonsxyz"].Message)
	})

	// teardown manager
	cancel()
	wg.Wait()
}

func TestErrorRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	logf.SetLogger(testr.New(t))

	wg := testManagerWithFakeClients(t, ctx)

	t.Run("should_be_able_to_reset_step", func(t *testing.T) {
		tf := testReconciler.Planner.Terraform.(*terraform.TerraformFake)

		// Run step that will fail.
		tf.DestroyMustFail()
		testCreateCR(t, testEnvironmentCR(testNSN, testSpecLocalDestroy()))

		got := testGetCRWhenConditionReady(t, testNSN)

		assert.Equal(t, 1, len(got.Status.Conditions), "number of Status.Conditions")
		assert.Equal(t, metav1.ConditionTrue, got.Status.Conditions[0].Status)
		assert.Equal(t, v1.ReasonFailed, got.Status.Conditions[0].Reason)
		assert.Equal(t, "0/1 ready, 0 running, 1 error(s)", got.Status.Conditions[0].Message)

		assert.Equal(t, 1, len(got.Status.Steps), "number of Status.Steps")
		assert.Equal(t, v1.StateError, got.Status.Steps["Destroy"].State, "Status.Steps[Destroy].State")
		assert.Equal(t, "did not receive response from terraform destroy", got.Status.Steps["Destroy"].Message)

		// Fix error and reset step.
		tf.DestroyMustSucceed()
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
