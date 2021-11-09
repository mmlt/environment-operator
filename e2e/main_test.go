package e2e

import (
	"context"
	"encoding/base64"
	"github.com/go-logr/stdr"
	clusteropsv1 "github.com/mmlt/environment-operator/api/v1"
	"github.com/mmlt/environment-operator/controllers"
	"github.com/mmlt/environment-operator/pkg/client/addon"
	"github.com/mmlt/environment-operator/pkg/client/azure"
	"github.com/mmlt/environment-operator/pkg/client/kubectl"
	"github.com/mmlt/environment-operator/pkg/client/terraform"
	"github.com/mmlt/environment-operator/pkg/cloud"
	"github.com/mmlt/environment-operator/pkg/cluster"
	"github.com/mmlt/environment-operator/pkg/plan"
	"github.com/mmlt/environment-operator/pkg/source"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"log"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
	"sync"
	"testing"
)

// TestMain instantiates the following vars for usage in tests.
var (
	cfg            *rest.Config
	k8sClient      client.Client
	testEnv        *envtest.Environment
	testReconciler *controllers.EnvironmentReconciler
)

// Tests use the following config.
var (
	// UseExistingCluster selects what k8s API Server is used when running tests.
	// When true the kubeconfig-current-context api server will be used.
	// When false the envtest apiserver will be used.
	useExistingCluster = false

	// AlwaysShowLog true shows log while running.
	// When false only logs of failed tests are shown.
	alwaysShowLog = true

	// Namespace and name for test resources.
	testNSN = types.NamespacedName{
		Namespace: "default",
		Name:      "env314",
	}
	// Labels for test resources.
	testLabels = labels.Set{
		"envop.example.com/env": "localenv",
	}

	testCtx = context.Background()
)

// TestMain sets-up a test API server, runs tests and tears down the API server.
func TestMain(m *testing.M) {
	if alwaysShowLog {
		logf.SetLogger(stdr.New(log.New(os.Stdout, "", log.Lshortfile|log.Ltime)))
		stdr.SetVerbosity(5)
	}

	// Setup.
	testEnv = &envtest.Environment{
		BinaryAssetsDirectory: "../testbin/bin",
		UseExistingCluster:    &useExistingCluster,
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	mustNotErr("starting testenv", err)

	err = corev1.AddToScheme(scheme.Scheme)
	mustNotErr("add to schema", err)
	err = clusteropsv1.AddToScheme(scheme.Scheme)
	mustNotErr("add to schema", err)

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	mustNotErr("creating client", err)

	// write kubeconfig to envtest
	// use an alias to access envtest during debugging: alias k="kubectl --kubeconfig e2e/envtest"
	kc, err := restConfigToKubeConfig(cfg)
	mustNotErr("convert to kubeconfig", err)
	err = ioutil.WriteFile("envtest", kc, 0644)
	mustNotErr("write kubeconfig to envtest", err)

	// Run.
	r := m.Run()

	// Teardown.
	err = testEnv.Stop()
	mustNotErr("stopping testenv", err)

	os.Exit(r)
}

// TestManagerWithFakeClients starts a Manager and Reconciler with the fake clients.
func testManagerWithFakeClients(t *testing.T, ctx context.Context, labelSet labels.Set) *sync.WaitGroup {
	t.Helper()

	// Setup manager (similar to controller.go)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:         scheme.Scheme,
		LeaderElection: false,
	})
	mustNotErr("new manager", err)

	testReconciler = &controllers.EnvironmentReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: mgr.GetEventRecorderFor("envop"),
		LabelSet: labelSet,
		Environ: map[string]string{
			"PATH": "/usr/local/bin", //kubectl-tmplt uses kubectl
		},
	}

	testReconciler.Sources = &source.Sources{
		RootPath: filepath.Join(os.TempDir(), "envop"),
		Log:      ctrl.Log.WithName("source"),
	}

	az := &azure.AZFake{}
	az.SetupFakeResults()
	tf := &terraform.TerraformFake{
		Log: ctrl.Log.WithName("tffake"),
	}
	tf.SetupFakeResults(map[string]interface{}{
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
	cl := &cloud.Fake{}
	kc := &kubectl.KubectlFake{}
	clc := cluster.Client{
		Client: testReconciler.Client,
		Labels: labelSet,
	}
	testReconciler.Planner = &plan.Planner{
		Terraform: tf,
		Kubectl:   kc,
		Azure:     az,
		Cloud:     cl,
		Addon:     &addon.Addon{},
		Client:    clc,
		Log:       ctrl.Log.WithName("planner"),
	}

	// Add reconciler to manager.
	err = testReconciler.SetupWithManager(mgr)
	mustNotErr("setup with manager", err)

	// Start manager.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = mgr.Start(ctx)
		mustNotErr("starting manager", err)
	}()

	return &wg
}

func mustNotErr(msg string, err error) {
	if err != nil {
		panic(msg + ": " + err.Error())
	}
}

// RestConfigToKubeConfig converts a client-go rest.Config to a kubeconfig text.
func restConfigToKubeConfig(cfg *rest.Config) ([]byte, error) {
	const clusterName = "local"
	const userName = "admin"

	c := clientcmdapi.Config{
		Clusters: []clientcmdapi.NamedCluster{
			{
				Name: clusterName,
				Cluster: clientcmdapi.Cluster{
					Server:                   cfg.Host,
					CertificateAuthorityData: cfg.CAData,
				},
			},
		},
		Contexts: []clientcmdapi.NamedContext{
			{
				Name: "default",
				Context: clientcmdapi.Context{
					Cluster:  clusterName,
					AuthInfo: userName,
				},
			},
		},
		AuthInfos: []clientcmdapi.NamedAuthInfo{
			{
				Name: userName,
				AuthInfo: clientcmdapi.AuthInfo{
					ClientCertificateData: cfg.CertData,
					ClientKeyData:         cfg.KeyData,
					Token:                 cfg.BearerToken,
					Username:              cfg.Username,
					Password:              cfg.Password,
				},
			},
		},
		CurrentContext: "default",
	}

	out, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}

	return out, nil

}
