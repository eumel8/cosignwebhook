test-cluster:
	@echo "Creating registry..."
	@k3d registry create registry.localhost --port 5000
	@echo "Adding registry to cluster..."
	@k3d cluster create cosign-tests --registry-use k3d-registry.localhost:5000

test-image:
	@echo "Building test image..."
	@docker build -t k3d-registry.localhost:5000/cosignwebhook:dev .
	@echo "Pushing test image..."
	@docker push k3d-registry.localhost:5000/cosignwebhook:dev
	@echo "Signing test image..."
	@cosign sign --tlog-upload=false --key cosign.key k3d-registry.localhost:5000/cosignwebhook:dev