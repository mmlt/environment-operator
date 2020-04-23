module github.com/mmlt/environment-operator

go 1.14

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/stdr v0.0.0-20190808155957-db4f46c40425
	github.com/hashicorp/hcl v1.0.0
	github.com/imdario/mergo v0.3.9
	github.com/mmlt/kubectl-tmplt v0.1.0
	github.com/mmlt/testr v0.0.0-20200331071714-d38912dd7e5a
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/otiai10/copy v1.1.1
	github.com/prometheus/client_golang v0.9.2
	github.com/stretchr/testify v1.5.1
	golang.org/x/mod v0.2.0
	golang.org/x/tools v0.0.0-20191119224855-298f0cb1881e
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	k8s.io/klog v1.0.0
	sigs.k8s.io/controller-runtime v0.4.0
)
