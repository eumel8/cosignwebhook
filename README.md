# Cosign Webhook

Kubernetes Validation Admission Controller to verify Cosign Image signatures.

<img src="cosignwebhook.png" alt="cosignwebhook" width="680"/>

Watch POD creating in deployments, looking for the first container image and a present RSA publik key to verify.

# Installation with Helm

```bash
helm -n cosignwebhook upgrade -i cosignwebhook oci://ghcr.io/eumel8/charts/cosignwebhook --versi
on 2.0.0 --create-namespace
```

this installation has some advantages:

* auto generate TLS key pair
* setup ServiceMonitor and GrafanaDashboard

# Installation with manifest

As Cluster Admin create a namespace and install the Admission Controller:

```bash
kubectl create namespace cosignwebhook
kubectl -n cosignwebhook apply -f manifests/rbac.yaml
kubectl -n cosignwebhook apply -f manifests/manifest.yaml
```

## Cert generation

```bash
generate-certs.sh --service cosignwebhook --webhook cosignwebhook --namespace cosignwebhook --secret cosignwebhook
```

# Usage

Add your Cosign public key as env var in container spec of the first container:

```yaml
        env:
        - name: COSIGNPUBKEY
          value: |
              -----BEGIN PUBLIC KEY-----
              MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEGOrnlJ1lFxAFTY2LF1vCuVHNZr9H
              QryRDinn+JhPrDYR2wqCP+BUkeWja+RWrRdmskA0AffxBzaQrN/SwZI6fA==
              -----END PUBLIC KEY-----
```

or create a secret and reference it in the deployment

```yaml
apiVersion: v1
data:
  COSIGNPUBKEY: LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0KTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFS1BhWUhnZEVEQ3ltcGx5emlIdkJ5UjNxRkhZdgppaWxlMCtFMEtzVzFqWkhJa1p4UWN3aGsySjNqSm5VdTdmcjcrd05DeENkVEdYQmhBSTJveE1LbWx3PT0KLS0tLS1FTkQgUFVCTElDIEtFWS0tLS0tkind: Secret
metadata:
  name: cosignwebhook
type: Opaque
```

```yaml
        env:
        - name: COSIGNPUBKEY
          valueFrom:
            secretKeyRef:
              name: cosignwebhook
              key: COSIGNPUBKEY
```

Note: The secret MUST be named `cosignwebhook` and the data values MIST be names `COSIGNPUBKEY`

# Test

Based on the signed image and the corresponding key, the demo app should appear or denied (check event log)

```bash
kubectl create namespace cosignwebhook
kubectl -n cosignwebhook apply -f manifests/demoapp.yaml
```

# TODO

* Support private images [x]
* Support multiple container/keys

## local build

```bash
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o cosignwebhook
```
## Credits

Frank Kloeker f.kloeker@telekom.de

Life is for sharing. If you have an issue with the code or want to improve it, feel free to open an issue or an pull request.

The Operator is inspired by [@pipo02mix](https://github.com/pipo02mix/grumpy), a good place
to learn fundamental things about Admission Controllert
