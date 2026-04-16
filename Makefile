# PORT is the registry's internal port (always 5000). k3s containerd mirrors
# are configured against it, so in-cluster image refs use this port.
# HOST_PORT is the host-side mapping into the registry container. On macOS,
# port 5000 is taken by AirPlay Receiver, so shell.nix overrides it to 5100.
PORT             := 5000
HOST_PORT        ?= 5000
BUSYBOX_SRC      := busybox:latest
BUSYBOX_DIGEST   := $(shell docker inspect --format='{{index .RepoDigests 0}}' $(BUSYBOX_SRC))
HOST_REGISTRY    := k3d-registry.localhost:$(HOST_PORT)
CLUSTER_REGISTRY := k3d-registry.localhost:$(PORT)
COSIGN           ?= cosign-v3
SIGNING_CONFIG   := test/framework/signing-config.json
BINARY           := cosignwebhook

#############
### BUILD ###
#############

.PHONY: build
build:
	@echo "Building binary locally..."
	@CGO_ENABLED=0 GOOS=linux go build -o $(BINARY) .

.PHONY: build-image
build-image: build
	@echo "Building dev image with local binary..."
	@docker build -f Dockerfile.dev -t $(HOST_REGISTRY)/$(BINARY):dev .

.PHONY: build-image-full
build-image-full:
	@echo "Building image with full Dockerfile..."
	@docker build -t $(HOST_REGISTRY)/$(BINARY):dev .

#############
### TESTS ###
#############

.PHONY: test-e2e
test-e2e:
	@echo "Running e2e tests..."
	@export COSIGN_E2E="42" && export REGISTRY_PORT="$(HOST_PORT)" && go test -v -race -count 1 ./test/

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@go test -v -race -count 1 ./webhook/

###########
### E2E ###
###########

e2e-cluster:
	@echo "Creating registry..."
	@k3d registry create registry.localhost --port $(HOST_PORT)
	@echo "Creating cluster..."
	@uname -m | grep -q 'Darwin' && export K3D_FIX_DNS=0; \
	if [ -f env/k3d-config.yaml ]; then \
		echo "Using k3d config from env/k3d-config.yaml"; \
		k3d cluster create --config env/k3d-config.yaml; \
	else \
		k3d cluster create cosign-tests --registry-use k3d-registry.localhost:$(HOST_PORT); \
	fi
	@echo "Create test namespace..."
	@kubectl create namespace test-cases

e2e-keys:
	@echo "Generating cosign keys..."
	@export COSIGN_PASSWORD="" && \
	 $(COSIGN) generate-key-pair && \
	 $(COSIGN) generate-key-pair --output-key-prefix second

# e2e-images: Full Docker build (for CI)
e2e-images:
	@echo "Checking for cosign.key..."
	@test -f cosign.key || (echo "cosign.key not found. Run 'make e2e-keys' to generate the pairs needed for the tests." && exit 1)
	@echo "Building test image (full build)..."
	@docker build -t $(HOST_REGISTRY)/$(BINARY):dev .
	@$(MAKE) _e2e-images-common

# e2e-images-dev: Fast local build (for development)
e2e-images-dev: build-image
	@echo "Checking for cosign.key..."
	@test -f cosign.key || (echo "cosign.key not found. Run 'make e2e-keys' to generate the pairs needed for the tests." && exit 1)
	@$(MAKE) _e2e-images-common

# Common image tasks (push, sign, busybox setup)
_e2e-images-common:
	@echo "Pushing test image..."
	@docker push $(HOST_REGISTRY)/$(BINARY):dev
	@echo "Signing test image..."
	@export COSIGN_PASSWORD="" && \
		$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key cosign.key `docker inspect --format='{{index .RepoDigests 0}}' $(HOST_REGISTRY)/$(BINARY):dev`
	@echo "Importing test image to cluster..."
	@k3d image import $(HOST_REGISTRY)/$(BINARY):dev --cluster cosign-tests
	@echo "Pulling busybox..."
	@docker pull $(BUSYBOX_SRC)
	@echo "Building distinct busybox images..."
	@echo 'FROM $(BUSYBOX_SRC)' | docker build --label="variant=first" -t $(HOST_REGISTRY)/busybox:first -
	@echo 'FROM $(BUSYBOX_SRC)' | docker build --label="variant=second" -t $(HOST_REGISTRY)/busybox:second -
	@echo "Pushing and signing busybox images..."
	@FIRST_DIGEST=$$(docker push $(HOST_REGISTRY)/busybox:first 2>&1 | grep -oP 'digest: \Ksha256:[a-f0-9]+'); \
	SECOND_DIGEST=$$(docker push $(HOST_REGISTRY)/busybox:second 2>&1 | grep -oP 'digest: \Ksha256:[a-f0-9]+'); \
	echo "Signing first: $(HOST_REGISTRY)/busybox@$$FIRST_DIGEST"; \
	export COSIGN_PASSWORD=""; \
	$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key cosign.key $(HOST_REGISTRY)/busybox@$$FIRST_DIGEST; \
	echo "Signing second: $(HOST_REGISTRY)/busybox@$$SECOND_DIGEST"; \
	export COSIGN_PASSWORD=""; \
	$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key second.key $(HOST_REGISTRY)/busybox@$$SECOND_DIGEST

e2e-deploy:
	@echo "Deploying test image..."
	@HELM_VALUES=""; \
	if [ -f env/dev.values.yaml ]; then \
		echo "Using values from env/dev.values.yaml"; \
		HELM_VALUES="-f env/dev.values.yaml"; \
	fi; \
	helm upgrade -i cosignwebhook chart -n cosignwebhook --create-namespace \
		$$HELM_VALUES \
		--set image.repository=$(CLUSTER_REGISTRY)/cosignwebhook \
		--set image.tag=dev \
		--set-file cosign.scwebhook.key=cosign.pub \
		--set logLevel=debug \
		--wait --debug --atomic

e2e-prep: e2e-cluster e2e-keys e2e-images-dev e2e-deploy

e2e-cleanup:
	@echo "Cleaning up test env..."
	@k3d registry delete registry.localhost || echo "Deleting k3d registry failed. Continuing..."
	@helm uninstall cosignwebhook -n cosignwebhook || echo "Uninstalling cosignwebhook helm release failed. Continuing..."
	@k3d cluster delete cosign-tests || echo "Deleting cosign tests k3d cluster failed. Continuing..."
	@rm -f cosign.pub cosign.key second.pub second.key || echo "Removing files failed. Continuing..."
	@echo "Done."

#############
### CHART ###
#############

.PHONY: chart-lint chart
chart-lint:
	@echo "Linting chart..."
	@helm lint chart

chart:
	@echo "Packaging chart..."
	@helm package chart
