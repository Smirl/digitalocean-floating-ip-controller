# digitalocean-floating-ip-controller

Assigns Kubernetes nodes to Digitalocean floating IPs

This project is intended to be run as a Kubernetes pod inside a
DOKS ([DigitalOcean Kubernetes][DOKS]) cluster.

It was inspired by [a similar `bash` controller][bash] by [@mwthink][mwthink].


## Getting Started

The controller will assign a Kubernetes node to a floating IP using the DigitalOcean API. A Custom Resource Definition called a `FloatingIPBinding` is
created for each floating IP that should be managed by the controller.

```yaml
apiVersion: digitalocean.smirlwebs.com/v1beta1
kind: FloatingIPBinding
metadata:
  name: main
spec:
  floatingIP: 123.10.10.10
  nodeSelectorPolicy: Newest
  nodeSelector:
    matchLabels:
      role: ingres
```

Full CRD API docs can be found at [docs.crds.dev][api].

## Node Selection

By default the `Newest` of all nodes is assigned to the floating IP as the
controller watches Nodes as well as `FloatingIPBinding`. This can be changed
by specifying a `nodeSelector` and/or a `nodeSelectorPolicy` in the object. 

Currently supported policies are:

- `Newest` _(default)_ - The newest node matching the selector
- `Oldest` - The oldest node matching the selector
- `Random` - A random node matching the selector


## Controller Deployment

### Installation

The controller can be deployed from this repo with:

```console
IMG=ghcr.io/smirl/digitalocean-floating-ip-controller:v0.1.0 make deploy
```

### Controller Configuration
You **must** provide the following as *environment variables*:
- `DO_TOKEN`
  - DigitalOcean API token

This is taken from a secret called `do-floating-ip-controller` which must be
added to the cluster.

## Contributing

Please feel free to raise an issue or pull request. Releases automatically
build and deploy to a test cluster. The github workflow requires the ServiceAccount
to be deployed into the cluster and the token added as a repository secret.

```console
kubectl apply -f serviceaccount.yaml
```

[DOKS]: https://www.digitalocean.com/products/kubernetes/
[bash]: https://github.com/mwthink/digitalocean-floating-ip-controller
[mwthink]: https://github.com/mwthink
[api]: https://doc.crds.dev/github.com/Smirl/digitalocean-floating-ip-controller
