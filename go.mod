module github.com/smirl/digitalocean-floating-ip-controller

go 1.15

require (
	github.com/digitalocean/godo v1.60.0
	github.com/go-logr/logr v0.3.0
	github.com/jarcoal/httpmock v1.0.8
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	go.uber.org/zap v1.15.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.7.2
)
