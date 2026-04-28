# NOTE: kind version is specified at aqua.yaml
# renovate:
KIND_IMAGE_REF := "v1.34.0@sha256:7416a61b42b1662ca6ca89f02028ac133a309a2a30ba309614e8ec94d976dc5a"
# renovate: datasource=github-releases depName=cert-manager/cert-manager
CERT_MANAGER_VERSION := 1.20.0
CERT_MANAGER_GPG_KEY_URL := https://cert-manager.io/public-keys/cert-manager-keyring-2021-09-20-1020CF3C033D4F35BAE1C19E1226061C665DF13E.gpg

PROJECT_DIR := $(CURDIR)/../
CHART_DIR := $(PROJECT_DIR)/tmp-charts

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
	aqua i -l

$(CHART_DIR):
	mkdir -p $(CHART_DIR)

$(CHART_DIR)/cert-manager-keyring.gpg: | $(CHART_DIR)
	curl -sSLf -o $@ $(CERT_MANAGER_GPG_KEY_URL)

$(CHART_DIR)/cert-manager/Chart.yaml: $(CHART_DIR)/cert-manager-keyring.gpg
	helm pull oci://quay.io/jetstack/charts/cert-manager --version v$(CERT_MANAGER_VERSION) --verify --keyring $(CHART_DIR)/cert-manager-keyring.gpg --untar --untardir $(CHART_DIR)

.PHONY: start
start: $(CHART_DIR)/cert-manager/Chart.yaml
	kind create cluster --name=$(KIND_CLUSTER_NAME) --config=$(KIND_CONFIG) --image=kindest/node:$(KIND_IMAGE_REF) --wait 1m
	$(MAKE) -C ../ docker-build
	kind load docker-image nyamber-controller:dev --name=$(KIND_CLUSTER_NAME)
	kind load docker-image localhost:5151/nyamber-runner:dev --name=$(KIND_CLUSTER_NAME)
	helm install cert-manager $(CHART_DIR)/cert-manager/ -n cert-manager --create-namespace --set crds.enabled=true
	kubectl -n cert-manager wait --for=condition=available --timeout=180s --all deployments
	kubectl apply -k ../config/default
	kubectl -n nyamber wait --for=condition=available --timeout=180s --all deployments

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
