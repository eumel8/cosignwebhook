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
	@echo "Adding registry to cluster..."
	@uname -m | grep -q 'Darwin' && export K3D_FIX_DNS=0; k3d cluster create cosign-tests --registry-use k3d-registry.localhost:$(HOST_PORT)
	@echo "Create test namespace..."
	@kubectl create namespace test-cases

e2e-keys:
	@echo "Generating cosign keys..."
	@export COSIGN_PASSWORD="" && \
	 $(COSIGN) generate-key-pair && \
	 $(COSIGN) generate-key-pair --output-key-prefix second

e2e-images:
	@echo "Checking for cosign.key..."
	@test -f cosign.key || (echo "cosign.key not found. Run 'make e2e-keys' to generate the pairs needed for the tests." && exit 1)
	@echo "Building test image..."
	@docker build -t $(HOST_REGISTRY)/cosignwebhook:dev .
	@echo "Pushing test image..."
	@docker push $(HOST_REGISTRY)/cosignwebhook:dev
	@echo "Signing test image..."
	@export COSIGN_PASSWORD="" && \
		$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key cosign.key `docker inspect --format='{{index .RepoDigests 0}}' $(HOST_REGISTRY)/cosignwebhook:dev`
	@echo "Importing test image to cluster..."
	@k3d image import $(HOST_REGISTRY)/cosignwebhook:dev --cluster cosign-tests
	@echo "Pulling busybox..."
	@docker pull $(BUSYBOX_SRC)
	@echo "Tagging busybox images..."
	@docker tag $(BUSYBOX_SRC) $(HOST_REGISTRY)/busybox:first
	@docker tag $(BUSYBOX_SRC) $(HOST_REGISTRY)/busybox:second
	@echo "Pushing busybox images..."
	@docker push $(HOST_REGISTRY)/busybox --all-tags
	@echo "Signing busybox images..."
	FIRST_DIGEST=$$(docker inspect --format='{{index .RepoDigests 1}}' $(HOST_REGISTRY)/busybox:first); \
	SECOND_DIGEST=$$(docker inspect --format='{{index .RepoDigests 1}}' $(HOST_REGISTRY)/busybox:second); \
	echo "Signing first: $$FIRST_DIGEST"; \
	export COSIGN_PASSWORD=""; \
	$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key cosign.key `docker inspect --format='{{index .RepoDigests 1}}' $(HOST_REGISTRY)/busybox:first`; \
	echo "Signing second: $$SECOND_DIGEST"; \
	export COSIGN_PASSWORD=""; \
	$(COSIGN) sign --signing-config=$(SIGNING_CONFIG) --allow-http-registry --allow-insecure-registry --key second.key `docker inspect --format='{{index .RepoDigests 1}}' $(HOST_REGISTRY)/busybox:second`

e2e-deploy:
	@echo "Deploying test image..."
	@helm upgrade -i cosignwebhook chart -n cosignwebhook --create-namespace \
		--set image.repository=$(CLUSTER_REGISTRY)/cosignwebhook \
		--set image.tag=dev \
		--set-file cosign.scwebhook.key=cosign.pub \
		--set logLevel=debug \
		--wait --debug --atomic

e2e-prep: e2e-cluster e2e-keys e2e-images e2e-deploy

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
