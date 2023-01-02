# grumpy

Kubernetes Validation Admission Controller example. I have writen down [a guide explaining how it is has been built and how to run it](https://docs.giantswarm.io/guides/creating-your-own-admission-controller).

# updates

## local build

```bash
CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o grumpywebhook
```

## cert generation

```bash
generate-certs.sh --service grumpy --webhook grumpy --namespace grumpy --secret grumpy
```


