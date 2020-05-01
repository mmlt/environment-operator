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
	"github.com/mmlt/environment-operator/pkg/infra"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"github.com/mmlt/environment-operator/pkg/terraform"
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
	cfg *rest.Config
	k8sClient client.Client
	testEnv *envtest.Environment

	// TestReconciler is the reconciler under test.
	testReconciler *EnvironmentReconciler
)

func TestE2E(t *testing.T) {
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

	By("starting testenv")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	if debugging {
		ctrl.Log.Info("API Server", "host", cfg.Host)
	}

	err = clusteropsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	// Setup manager (similar to main.go)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// Create environment reconciler and all it's dependencies.
	testReconciler = &EnvironmentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("envctrl"),
		Log:    ctrl.Log.WithName("EnvironmentReconciler"),
		//TODO Selector: *selector,
	}
	testReconciler.Sources = &source.Sources{
		BasePath: filepath.Join(os.TempDir(), "envrecon"),
		Log:      testReconciler.Log.WithName("source"),
	}
	testReconciler.Plan = &plan.Plan{
		Log: testReconciler.Log.WithName("plan"),
	}
	tf := &terraform.TerraformFake{
		Log: testReconciler.Log.WithName("tffake"),
	}
	tf.SetupFakeResults()
	testReconciler.Executor = &infra.Executor{
		UpdateSink: testReconciler,
		EventSink:  testReconciler,
		Terraform:  tf,
		Log:        testReconciler.Log.WithName("executor"),
	}

	// Add reconciler to manager.
	err = testReconciler.SetupWithManager(mgr)
	Expect(err).ToNot(HaveOccurred())

	// Start manager.
	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, testTimeoutSec)

var _ = AfterSuite(func() {
	By("tearing down testenv")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
