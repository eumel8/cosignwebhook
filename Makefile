PORT := 5000

#############
### TESTS ###
#############

.PHONY: test-e2e
test-e2e:
	@echo "Running e2e tests..."
	@export COSIGN_E2E="42" && go test -v -race -count 1 ./test/

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@go test -v -race -count 1 ./webhook/

###########
### E2E ###
###########

e2e-cluster:
	@echo "Creating registry..."
	@k3d registry create registry.localhost --port $(PORT)
	@echo "Adding registry to cluster..."
	@uname | grep -q 'Darwin' && export K3D_FIX_DNS=0; k3d cluster create cosign-tests --registry-use k3d-registry.localhost:$(PORT)
	@echo "Create test namespace..."
	@kubectl create namespace test-cases

e2e-keys:
	@echo "Generating cosign keys..."
	@export COSIGN_PASSWORD="" && \
	 cosign generate-key-pair && \
	 cosign generate-key-pair --output-key-prefix second

e2e-images:
	@echo "Checking for cosign.key..."
	@test -f cosign.key || (echo "cosign.key not found. Run 'make e2e-keys' to generate the pairs needed for the tests." && exit 1)
	@echo "Building test image..."
	@docker build -t k3d-registry.localhost:$(PORT)/cosignwebhook:dev .
	@echo "Pushing test image..."
	@docker push k3d-registry.localhost:$(PORT)/cosignwebhook:dev
	@echo "Signing test image..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:$(PORT)/cosignwebhook:dev
	@echo "Importing test image to cluster..."
	@k3d image import k3d-registry.localhost:$(PORT)/cosignwebhook:dev --cluster cosign-tests
	@echo "Building busybox image..."
	@docker pull busybox:latest
	@echo "Tagging & pushing busybox images..."
	@docker tag busybox:latest k3d-registry.localhost:$(PORT)/busybox:first
	@docker tag busybox:latest k3d-registry.localhost:$(PORT)/busybox:second
	@docker push k3d-registry.localhost:$(PORT)/busybox --all-tags
	@echo "Signing busybox images..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:$(PORT)/busybox:first && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:$(PORT)/busybox:first && \
		cosign sign --tlog-upload=false --key second.key k3d-registry.localhost:$(PORT)/busybox:second

e2e-deploy:
	@echo "Deploying test image..."
	@helm upgrade -i cosignwebhook chart -n cosignwebhook --create-namespace \
		--set image.repository=k3d-registry.localhost:$(PORT)/cosignwebhook \
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
