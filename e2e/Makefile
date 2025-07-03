# NOTE: kind version is specified at aqua.yaml
KIND_IMAGE_REF := "v1.33.1@sha256:050072256b9a903bd914c0b2866828150cb229cea0efe5892e2b644d5dd3b34f"
E2ETEST_K8S_VERSION := $(shell echo $(KIND_IMAGE_REF) | awk -F '[@v]' '{print $$2}')
CERT_MANAGER_VERSION := 1.18.1

PROJECT_DIR := $(CURDIR)/../
BIN_DIR := $(PROJECT_DIR)/bin

CURL := curl -sSLf
KUBECTL := $(BIN_DIR)/kubectl

KIND_CLUSTER_NAME := nyamber
KIND_CONFIG := kind-config.yaml
export KUBECONFIG

.PHONY: help
help:
	@echo "Choose one of the following target"
	@echo
	@echo "setup      Setup tools"
	@echo "start      Start kind cluster and install accurate"
	@echo "test       Run e2e tests"
	@echo "logs       Save logs as logs.tar.gz"
	@echo "stop       Stop the kind cluster"

.PHONY: setup
setup:
	mkdir -p $(BIN_DIR)
	$(CURL) -o $(BIN_DIR)/kubectl https://dl.k8s.io/release/v$(E2ETEST_K8S_VERSION)/bin/linux/amd64/kubectl && chmod a+x $(BIN_DIR)/kubectl

.PHONY: start
start:
	kind create cluster --name=$(KIND_CLUSTER_NAME) --config=$(KIND_CONFIG) --image=kindest/node:$(KIND_IMAGE_REF) --wait 1m
	$(MAKE) -C ../ docker-build
	kind load docker-image nyamber-controller:dev --name=$(KIND_CLUSTER_NAME)
	kind load docker-image localhost:5151/nyamber-runner:dev --name=$(KIND_CLUSTER_NAME)
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/download/v$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(KUBECTL) -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	$(KUBECTL) apply -k ../config/default
	$(KUBECTL) -n nyamber wait --for=condition=available --timeout=180s --all deployments

.PHONY: test
test:
	env RUN_E2E=1 \
		go test -v -race . -ginkgo.progress -ginkgo.v -ginkgo.fail-fast

.PHONY: logs
logs:
	rm -rf logs.tar.gz logs
	kind export logs --name=$(KIND_CLUSTER_NAME) ./logs
	tar czf logs.tar.gz logs
	rm -rf logs

.PHONY: stop
stop:
	kind delete cluster --name=$(KIND_CLUSTER_NAME)
	-docker image rm nyamber-controller:dev
	-docker image rm localhost:5151/nyamber-runner:dev
	-docker image prune -f
