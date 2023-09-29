e2e-cluster:
	@echo "Creating registry..."
	@k3d registry create registry.localhost --port 5000
	@echo "Adding registry to cluster..."
	@k3d cluster create cosign-tests --registry-use k3d-registry.localhost:5000
	@echo "Create test namespace..."
	@kubectl create namespace test-cases

e2e-keys:
	@echo "Generating cosign keys..."
	@export COSIGN_PASSWORD="" && \
	 cosign generate-key-pair && \
	 cosign generate-key-pair --output-key-prefix second

e2e-images:
	@echo "Checking for cosign.key..."
	@test -f cosign.key || (echo "cosign.key not found. Run 'make generate-key' to generate one." && exit 1)
	@echo "Building test image..."
	@docker build -t k3d-registry.localhost:5000/cosignwebhook:dev .
	@echo "Pushing test image..."
	@docker push k3d-registry.localhost:5000/cosignwebhook:dev
	@echo "Signing test image..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:5000/cosignwebhook:dev
	@echo "Importing test image to cluster..."
	@k3d image import k3d-registry.localhost:5000/cosignwebhook:dev --cluster cosign-tests
	@echo "Building busybox image..."
	@docker pull busybox:latest
	@echo "Tagging & pushing busybox images..."
	@docker tag busybox:latest k3d-registry.localhost:5000/busybox:first
	@docker tag busybox:latest k3d-registry.localhost:5000/busybox:second
	@docker push k3d-registry.localhost:5000/busybox --all-tags
	@echo "Signing busybox images..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:5000/busybox:first && \
		cosign sign --tlog-upload=false --key second.key k3d-registry.localhost:5000/busybox:second

e2e-deploy:
	@echo "Deploying test image..."
	@helm upgrade -i cosignwebhook chart -n cosignwebhook --create-namespace \
		--set image.repository=k3d-registry.localhost:5000/cosignwebhook \
		--set image.tag=dev \
		--set-file cosign.scwebhook.key=cosign.pub \
		--set logLevel=debug \
		--wait --debug

.PHONY: test-e2e
test-e2e:
	@echo "Running e2e tests..."
	@go test -race -count 1 ./test/

.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	@go test -race -count 1 ./webhook/

test-cleanup:
	@echo "Cleaning up..."
	@helm uninstall cosignwebhook -n cosignwebhook
	@k3d registry delete k3d-registry.localhost
	@k3d cluster delete cosign-tests