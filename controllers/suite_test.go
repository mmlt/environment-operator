/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"github.com/go-logr/stdr"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/executor"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"log"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"

	clusteropsv1 "github.com/mmlt/environment-operator/api/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	// +kubebuilder:scaffold:imports
)

// Debugging is a switch to increase time-outs and always show log output.
const debugging = true
const testTimeoutSec = 600

// Vars accessible from test cases.
var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment

	// TestReconciler is the reconciler under test.
	testReconciler *EnvironmentReconciler
)

// TestE2EWithFakes runs a test suite using envtest and fake clients.
func TestE2EWithFakes(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	if debugging {
		// Always show log output
		logf.SetLogger(stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime)))
		stdr.SetVerbosity(5)
	} else {
		logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))
	}

	By("setting up the test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	if debugging {
		ctrl.Log.Info("API Server", "host", cfg.Host)
	}

	err = clusteropsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Setup manager (similar to main.go)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).NotTo(HaveOccurred())

	// Create environment reconciler and all it's dependencies.
	testReconciler = &EnvironmentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("envop"),
		Log:      ctrl.Log.WithName("recon"),
		//TODO Selector: *selector,
	}
	testReconciler.Sources = &source.Sources{
		RootPath: filepath.Join(os.TempDir(), "envop"),
		Log:      testReconciler.Log.WithName("source"),
	}
	tf := &terraform.TerraformFake{
		Log: testReconciler.Log.WithName("tffake"),
	}
	tf.SetupFakeResults(map[string]interface{}{
		"one": map[string]interface{}{
			"kube_admin_config": map[string]interface{}{
				"client_certificate":     cfg.CertData,
				"client_key":             cfg.KeyData,
				"cluster_ca_certificate": cfg.CAData,
				"host":                   cfg.Host,
				"password":               cfg.Password,
				"username":               cfg.Username,
			},
		},
	})
	kc := &kubectl.KubectlFake{}
	az := &azure.AZFake{}
	az.SetupFakeResults()
	testReconciler.Planner = &plan.Planner{
		Terraform: tf,
		Kubectl:   kc,
		Azure:     az,
		Addon: &addon.Addon{
			Log: testReconciler.Log.WithName("addon"),
		},
		Log: testReconciler.Log.WithName("planner"),
	}
	testReconciler.Executor = &executor.Executor{
		UpdateSink: testReconciler,
		EventSink:  testReconciler,
		Log:        testReconciler.Log.WithName("executor"),
	}

	// Add reconciler to manager.
	err = testReconciler.SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	// Start manager.
	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).NotTo(HaveOccurred())
	}()

	close(done)
}, testTimeoutSec)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
