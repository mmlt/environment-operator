package controllers

import (
	"context"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/executor"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = ginkgo.Describe("Happy path tests", func() {
	// namespace/name of the resource used for testing.
	nsn := types.NamespacedName{
		Name:      "env314",
		Namespace: "default",
	}

	ginkgo.BeforeEach(func() {
		// failed test runs that don't clean up leave resources behind.
		/*		keys := []string{aclKeyName, secretsKeyName}
				for _, value := range keys {
					apiClient.Secrets().DeleteSecretScope(value)
				}*/
	})

	ginkgo.AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
	})

	//ginkgo.Context("Happy path", func() {
	ginkgo.It("Should provision infra", func() {
		toCreate := testEnvironmentCR(nsn, testSpecLocal())

		ginkgo.By("Creating kind Environment and waiting for reconcile completion")
		Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

		fetched := &v1.Environment{}
		Eventually(func() bool {
			k8sClient.Get(context.Background(), nsn, fetched)
			for _, c := range fetched.Status.Conditions {
				if c.Type == "Ready" {
					return c.Status == metav1.ConditionTrue
				}
			}
			return false
		}, testTimeoutSec, time.Second).Should(BeTrue())

		ginkgo.By("Check hat the reconcile doesn't continue (no more steps are started)")
		c := testutil.ToFloat64(executor.MetricSteps)
		time.Sleep(time.Second)
		Expect(testutil.ToFloat64(executor.MetricSteps) - c).To(Equal(0.0))

		ginkgo.By("Checking CR Status")
		Expect(len(fetched.Status.Steps)).To(BeNumerically("==", 7))

		Expect(fetched.Status.Steps["Init"].State).To(Equal(v1.StateReady))
		Expect(fetched.Status.Steps["Init"].Message).To(Equal("terraform init errors=0 warnings=0"))

		Expect(fetched.Status.Steps["Plan"].State).To(Equal(v1.StateReady))
		Expect(fetched.Status.Steps["Plan"].Message).To(Equal("terraform plan errors=0 warnings=0 added=1 changed=2 deleted=1"))

		Expect(fetched.Status.Steps["Apply"].State).To(Equal(v1.StateReady))
		Expect(fetched.Status.Steps["Apply"].Message).To(Equal("terraform apply errors=0 added=1 changed=2 deleted=1"))

		Expect(fetched.Status.Steps["AKSPoolone"].State).To(Equal(v1.StateReady))

		Expect(fetched.Status.Steps["Kubeconfigone"].State).To(Equal(v1.StateReady))

		Expect(fetched.Status.Steps["AKSAddonPreflightone"].State).To(Equal(v1.StateReady))

		Expect(fetched.Status.Steps["Addonsone"].State).To(Equal(v1.StateReady))

	})
	//})
})
