KUBERNETES_VERSION = 1.23.4

KIND_CONFIG = kind-config.yaml
export KUBECONFIG

.PHONY: help
help:
	@echo "Choose one of the following target"
	@echo
	@echo "start      Start kind cluster and install accurate"
	@echo "test       Run e2e tests"
	@echo "logs       Save logs as logs.tar.gz"
	@echo "stop       Stop the kind cluster"

.PHONY: start
start:
	kind create cluster --name=nyamber --config=$(KIND_CONFIG) --image=kindest/node:v$(KUBERNETES_VERSION) --wait 1m
	$(MAKE) -C ../ docker-build IMG=nyamber:dev
	kind load docker-image nyamber:dev --name=nyamber
	kubectl apply -k ../config/default
	kubectl -n nyamber wait --for=condition=available --timeout=180s --all deployments

.PHONY: test
test:
	env RUN_E2E=1 \
		go test -v -race . -ginkgo.progress -ginkgo.v -ginkgo.failFast

.PHONY: logs
logs:
	rm -rf logs.tar.gz logs
	kind export logs --name=nyamber ./logs
	tar czf logs.tar.gz logs
	rm -rf logs

.PHONY: stop
stop: 
	kind delete cluster --name=nyamber
	-docker image rm nyamber:dev
	-docker image prune -f
