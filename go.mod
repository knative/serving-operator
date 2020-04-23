module knative.dev/serving-operator

go 1.13

require (
	cloud.google.com/go v0.52.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.6.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.12.9-0.20191108183826-59d068f8d8ff // indirect
	github.com/aws/aws-sdk-go v1.28.7 // indirect
	github.com/go-logr/zapr v0.1.1
	github.com/go-openapi/spec v0.19.5
	github.com/go-openapi/swag v0.19.6 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/manifestival/client-go-client v0.2.0
	github.com/manifestival/manifestival v0.5.0
	github.com/openzipkin/zipkin-go v0.2.2 // indirect
	github.com/operator-framework/operator-sdk v0.15.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.9.1 // indirect
	github.com/prometheus/procfs v0.0.8 // indirect
	go.opencensus.io v0.22.2
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20200117160349-530e935923ad // indirect
	golang.org/x/sys v0.0.0-20200122134326-e047566fdf82 // indirect
	golang.org/x/tools v0.0.0-20200123022218-593de606220b // indirect
	google.golang.org/genproto v0.0.0-20200122232147-0452cf42e150 // indirect
	gopkg.in/yaml.v2 v2.2.7 // indirect
	istio.io/api v0.0.0-20191115173247-e1a1952e5b81
	istio.io/client-go v0.0.0-20191120150049-26c62a04cdbc
	istio.io/gogo-genproto v0.0.0-20191029161641-f7d19ec0141d // indirect
	k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver v0.16.4 // indirect
	k8s.io/apimachinery v0.16.5-beta.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.16.4
	k8s.io/kube-openapi v0.0.0-20190918143330-0270cf2f1c1d
	k8s.io/utils v0.0.0-20200122174043-1e243dd1a584 // indirect
	knative.dev/caching v0.0.0-20200122154023-853d6022845c
	knative.dev/pkg v0.0.0-20200410152005-2a1db869228c
	knative.dev/test-infra v0.0.0-20200423234049-f645de8d9a50
	sigs.k8s.io/yaml v1.1.0
)

replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/apiserver => k8s.io/apiserver v0.16.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
	k8s.io/component-base => k8s.io/component-base v0.16.4
	k8s.io/cri-api => k8s.io/cri-api v0.16.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.4
	k8s.io/kubectl => k8s.io/kubectl v0.16.4
	k8s.io/kubelet => k8s.io/kubelet v0.16.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.4
	k8s.io/metrics => k8s.io/metrics v0.16.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.4
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309 // Required by Helm
