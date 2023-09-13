test-create-cluster:
	@echo "Creating registry..."
	@k3d registry create registry.localhost --port 5000
	@echo "Adding registry to cluster..."
	@k3d cluster create cosign-tests --registry-use k3d-registry.localhost:5000
	@echo "Create test namespace..."
	@kubectl create namespace test-cases

test-generate-key:
	@echo "Generating key..."
	@export COSIGN_PASSWORD="" && \
	 cosign generate-key-pair && \
	 mv cosign.key test/keys/cosign.key && \
	 mv cosign.pub test/keys/cosign.pub

test-busybox-images:
	@echo "Building busybox image..."
	@docker pull busybox:latest
	@echo "Tagging & pushing busybox images..."
	@docker tag busybox:latest k3d-registry.localhost:5000/busybox:latest
	@docker tag busybox:latest k3d-registry.localhost:5000/busybox:second
	@docker push k3d-registry.localhost:5000/busybox --all-tags
	@echo "Signing busybox images..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key test/keys/cosign.key k3d-registry.localhost:5000/busybox:latest && \
		cosign sign --tlog-upload=false --key test/keys/second.key k3d-registry.localhost:5000/busybox:second


test-image:
	@echo "Checking for cosign.key..."
	@test -f test/keys/cosign.key || (echo "cosign.key not found. Run 'make generate-key' to generate one." && exit 1)
	@echo "Building test image..."
	@docker build -t k3d-registry.localhost:5000/cosignwebhook:dev .
	@echo "Pushing test image..."
	@docker push k3d-registry.localhost:5000/cosignwebhook:dev
	@echo "Signing test image..."
	@export COSIGN_PASSWORD="" && \
		cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:5000/cosignwebhook:dev

test-deploy:
	@echo "Deploying test image..."
	@SHA=$(shell docker inspect --format='{{index .RepoDigests 0}}' k3d-registry.localhost:5000/cosignwebhook:dev | cut -d '@' -f 2) && \
		echo "Using image SHA: $${SHA}" && \
		helm upgrade -i cosignwebhook chart -n cosignwebhook --create-namespace \
		--set image.repository=k3d-registry.localhost:5000/cosignwebhook \
		--set image.tag="dev@$${SHA}" \
		--set-file cosign.scwebhook.key=cosign.pub \
		--set logLevel=debug

test-cleanup:
	@echo "Cleaning up..."
	@helm uninstall cosignwebhook -n cosignwebhook
	@k3d registry delete k3d-registry.localhost
	@k3d cluster delete cosign-tests
