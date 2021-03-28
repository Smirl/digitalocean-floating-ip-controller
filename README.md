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

TODO

### Controller Configuration
You **must** provide the following as *environment variables*:
- `DO_TOKEN`
  - DigitalOcean API token

[DOKS]: https://www.digitalocean.com/products/kubernetes/
[bash]: https://github.com/mwthink/digitalocean-floating-ip-controller
[mwthink]: https://github.com/mwthink
[api]: https://doc.crds.dev/github.com/Smirl/digitalocean-floating-ip-controller