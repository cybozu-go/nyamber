# Image URL to use all building/pushing image targets
CONTROLLER_IMG ?= nyamber-controller:dev
RUNNER_IMG ?= localhost:5151/nyamber-runner:dev

##@ Build Dependencies
LOCALBIN ?= $(shell pwd)/bin

## Location to install dependencies to
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# the kubebuilder version of the ready-to-use can get by "./bin/setup-envtest list" command.
# renovate:
ENVTEST_K8S_VERSION = 1.34.1

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

# Generate manifests and code, and check if diff exists. When there are differences stop CI.
# To avoid CI stopping, edit annotation "controller-gen.kubebuilder.io/version:" in
# existing "nyamber.cybozu.io_virtualdcs.yaml" and "nyamber.cybozu.io_virtualdcs.yaml".
# both version must equal CONTROLLER_TOOLS_VERSION in Makefile.
.PHONY: check-generate
check-generate: ## Generate manifests and code, and check if diff exists.
	$(MAKE) manifests
	$(MAKE) generate
	$(MAKE) apidoc
	git diff --exit-code --name-only

.PHONY: test
test: ## Run tests.
	staticcheck ./...
	KUBEBUILDER_ASSETS="$(shell setup-envtest --arch=amd64 use $(ENVTEST_K8S_VERSION) -p path)" go test -v ./... -coverprofile cover.out

.PHONY: apidoc
apidoc: $(wildcard api/*/*_types.go)
	crd-to-markdown -f api/v1beta1/virtualdc_types.go -n VirtualDC > docs/crd_virtualdc.md
	crd-to-markdown -f api/v1beta1/autovirtualdc_types.go -n AutoVirtualDC > docs/crd_autovirtualdc.md

##@ Build
.PHONY: build
build: generate ## Build all binaries.
	go build -o $(LOCALBIN)/ -trimpath ./cmd/entrypoint
	go build -o $(LOCALBIN)/ -trimpath ./cmd/nyamber-controller

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	DOCKER_BUILDKIT=1 docker build -t ${CONTROLLER_IMG} .
	DOCKER_BUILDKIT=1 docker build -t ${RUNNER_IMG} -f ./Dockerfile.runner .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${CONTROLLER_IMG}
	docker push ${RUNNER_IMG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: start
start:
	ctlptl apply -f ./cluster.yaml
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/latest/download/cert-manager.yaml
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments

.PHONY: stop
stop:
	ctlptl delete -f ./cluster.yaml

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	kustomize build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kustomize build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && kustomize edit set image controller=${CONTROLLER_IMG}
	kustomize build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	kustomize build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -
