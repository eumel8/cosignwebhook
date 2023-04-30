# Navlinks Webhook

Kubernetes Admission Controller to create Navlinks in Rancher for Prometheus resource

Watch Prometheus creating in cluster

# Installation with Helm

```bash
helm -n navlinkswebhook upgrade -i navlinkswebhook oci://ghcr.io/caas/charts/navlinkswebhook --version 0.0.1 --create-namespace
```

# Usage

Create `Prometheusi` resource in cluster and the Admission Controller will install Navlinks to navigate to Monitoring resources

## local build

```bash
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o navlinkswebhook
```
## Credits

Frank Kloeker f.kloeker@telekom.de

Life is for sharing. If you have an issue with the code or want to improve it, feel free to open an issue or an pull request.

The Operator is inspired by [@pipo02mix](https://github.com/pipo02mix/grumpy), a good place
to learn fundamental things about Admission Controllert
# navlinkswebhook
