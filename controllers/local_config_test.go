package controllers

import (
	"context"
	v1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/pkg/infra"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"

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

	// TODO Add Tests for OpenAPI validation (or additional CRD features) specified in
	// your API definition.
	// Avoid adding tests for vanilla CRUD operations because they would
	// test Kubernetes API server, which isn't the goal here.

	//ginkgo.Context("Happy path", func() {
		ginkgo.It("Should provision infra", func() {
			toCreate := testEnvironmentCR(nsn, testSpecPlayground())

			ginkgo.By("Creating kind Environment and waiting for reconcile completion")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())

			fetched := &v1.Environment{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), nsn, fetched)
				//TODO return fetched.Status.Synced != v1.SyncedUnknown
				return fetched.Status.Synced == v1.SyncedReady && len(fetched.Status.Conditions) >= 3
			}, testTimeoutSec, time.Second).Should(BeTrue())

			ginkgo.By("Check hat the reconcile doesn't continue (no more steps are started)")
			c := testutil.ToFloat64(infra.MetricSteps)
			time.Sleep(time.Second)
			Expect(testutil.ToFloat64(infra.MetricSteps) - c).To(Equal(0.0))

			ginkgo.By("Checking CR Status")
			Expect(len(fetched.Status.Conditions)).To(BeNumerically("==", 3))
			Expect(fetched.Status.Conditions[0].Type).To(Equal("InfraInit"))
			Expect(fetched.Status.Conditions[0].Reason).To(Equal(v1.ReasonReady))
			Expect(fetched.Status.Conditions[0].Message).To(Equal("terraform init errors=0 warnings=0"))
			Expect(fetched.Status.Conditions[1].Type).To(Equal("InfraPlan"))
			Expect(fetched.Status.Conditions[1].Reason).To(Equal(v1.ReasonReady))
			Expect(fetched.Status.Conditions[1].Message).To(Equal("terraform plan errors=0 warnings=0 added=1 changed=2 deleted=1"))
			Expect(fetched.Status.Conditions[2].Type).To(Equal("InfraApply"))
			Expect(fetched.Status.Conditions[2].Reason).To(Equal(v1.ReasonReady))
			Expect(fetched.Status.Conditions[2].Message).To(Equal("terraform apply errors=0 added=1 changed=2 deleted=1"))
		})
	//})
})
