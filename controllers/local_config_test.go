package controllers

import (
	"context"
	v1 "github.com/mmlt/environment-operator/api/v1"

	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = ginkgo.Describe("Infrastructure dry-run", func() {

	const timeout = time.Second * 10
	const interval = time.Second * 1

	// CRNN is the namespace/name of the custom resource.
	crNN := types.NamespacedName{
		Name:      "testenv",
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

	ginkgo.Context("Happy path", func() {
		ginkgo.It("Should ...", func() {
			toCreate := crLocal(crNN)

			ginkgo.By("Creating CR")
			Expect(k8sClient.Create(context.Background(), toCreate)).Should(Succeed())
			//time.Sleep(time.Second * 5) //TODO

			fetched := &v1.Environment{}
			Eventually(func() bool {
				k8sClient.Get(context.Background(), crNN, fetched)
				return fetched.Status.Peek != v1.PeekUnknown
			}, timeout, interval).Should(BeTrue())

			//ginkgo.By("Checking CR Status")
			//Expect(len(fetched.Status.Conditions)).To(BeNumerically("==", 3))
			//Expect(fetched.Status.Conditions[0].Message).To(BeEmpty())
			//Expect(fetched.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			//Expect(fetched.Status.Conditions[1].Message).To(BeEmpty())
			//Expect(fetched.Status.Conditions[1].Status).To(Equal(metav1.ConditionTrue))
			//Expect(fetched.Status.Conditions[2].Message).To(BeEmpty())
			//Expect(fetched.Status.Conditions[2].Status).To(Equal(metav1.ConditionTrue))
			//Expect(fetched.Status.Synced).To(Equal(metav1.ConditionTrue))
		})
	})
})

func crLocal(nn types.NamespacedName) *v1.Environment {
	spec := v1.EnvironmentSpec{
		Defaults: v1.ClusterSpec{
			Infrastructure: v1.InfrastructureSpec{
				Source: v1.SourceSpec{
					Type: "local",
					URL:  "../config/samples/terraform", // relative to dir containing this _test.go file.
				},
				Main: "main.tf.tmplt",
				Values: map[string]string{
					"first": "default",
				},
			},
		},
		Clusters: []v1.ClusterSpec{
			{
				Name: "cpe",
				Infrastructure: v1.InfrastructureSpec{
					Values: map[string]string{
						"first": "cluster",
					},
				},
			}, {
				Name: "second",
				Infrastructure: v1.InfrastructureSpec{
					Values: map[string]string{
						"first": "cluster",
					},
				},
			},
		},
	}

	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: spec,
	}
}
