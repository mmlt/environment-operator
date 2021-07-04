module github.com/mmlt/environment-operator

go 1.16

require (
	github.com/Jeffail/gabs/v2 v2.6.0
	github.com/Masterminds/sprig/v3 v3.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/stdr v0.3.0
	github.com/hashicorp/go-multierror v1.1.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mmlt/testr v0.0.0-20200331071714-d38912dd7e5a
	github.com/otiai10/copy v1.1.1
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/robfig/cron/v3 v3.0.0
	github.com/rodaine/hclencoder v0.0.0-20190213202847-fb9757bb536e
	github.com/securego/gosec/v2 v2.8.1
	github.com/spf13/cobra v1.1.3
	github.com/stretchr/testify v1.7.0
	golang.org/x/tools v0.1.3
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/cli-runtime v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/code-generator v0.21.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime v0.9.0
	sigs.k8s.io/controller-tools v0.6.0
	sigs.k8s.io/yaml v1.2.0
)
