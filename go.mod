module github.com/machinezone/configmapsecrets

go 1.12

require (
	cloud.google.com/go v0.43.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.0
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/googleapis/gnostic v0.3.0 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/onsi/gomega v1.5.0
	github.com/prometheus/client_golang v1.1.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190701094942-4def268fd1a4 // indirect
	golang.org/x/net v0.0.0-20190724013045-ca1201d0de80 // indirect
	k8s.io/api v0.0.0-20190731142925-739c7f7721ed
	k8s.io/apiextensions-apiserver v0.0.0-20190731184107-91cdd51c95ef // indirect
	k8s.io/apimachinery v0.0.0-20190731142807-035e418f1ad9
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.3.3 // indirect
	k8s.io/kube-openapi v0.0.0-20190722073852-5e22f3d471e6 // indirect
	k8s.io/utils v0.0.0-20190801114015-581e00157fb1 // indirect
	sigs.k8s.io/controller-runtime v0.0.0-00010101000000-000000000000
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190718183610-8e956561bbf5
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.2.0-beta.4
)
