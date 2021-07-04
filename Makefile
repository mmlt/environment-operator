# Version (overridden during CD with repo semver tag)
VERSION ?= latest
# Image URL to use all building/pushing image targets
IMG ?= envop:${VERSION}
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the install-tools target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec


all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out
	go tool cover -func cover.out | tail -n 1

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go controller

# Install CRDs into a cluster
install: manifests
	kubectl apply -k config/crd

# Uninstall CRDs from a cluster
uninstall: manifests
	kubectl delete -k config/crd

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
#deploy: manifests
#	cd config/manager && kustomize edit set image controller=${IMG}
#	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	bin/controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...
	bin/gosec -quiet -exclude=G204,G304,G401,G505 ./...

# Generate code
generate-controller:
	bin/controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./..."

generate-clientgo:
	# pre-requisite: the repo is cloned at github.com/mmlt/environment-operator
	bin/client-gen --clientset-name versioned --input-base "" --input github.com/mmlt/environment-operator/api/clusterops/v1 --output-package github.com/mmlt/environment-operator/pkg/generated/clientset --go-header-file ./hack/boilerplate.go.txt --output-base ../../..
	bin/informer-gen --input-dirs github.com/mmlt/environment-operator/api/clusterops/v1 --versioned-clientset-package github.com/mmlt/environment-operator/pkg/generated/clientset/versioned --listers-package github.com/mmlt/environment-operator/pkg/generated/listers --output-package github.com/mmlt/environment-operator/pkg/generated/informers --go-header-file ./hack/boilerplate.go.txt --output-base ../../..

# Build the docker image
docker-build:
	docker build . -t ${IMG} --build-arg VERSION=${VERSION}

# Push the docker image
docker-push:
	docker push ${IMG}

# Push the docker image to local registry
docker-push-local:
	docker tag ${IMG} localhost:32000/${IMG}
	docker push localhost:32000/${IMG}

# Install tools.
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
install-tools:
	# tools of which the version is defined in go.mod
	grep _ pkg/internal/tools/tools.go | cut -d'"' -f2 | xargs -n1 go build -o bin/
	# envtest
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.3/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR)



gogenerate:
	go generate


