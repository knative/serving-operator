# Knative Serving Operator

Knative Serving Operator is a project aiming to deploy and manage Knative
Serving in an automated way.

The following will install [Knative Serving](https://github.com/knative/serving)
and configure it appropriately for your cluster in the `knative-serving`
namespace:

```
kubectl apply -f config/crds/serving_v1alpha1_knativeserving_crd.yaml
```

To install the operator from the source code, run the command:

```
ko apply -f config/
```

To install the operator from an existing image, change the value of `image` into
`quay.io/openshift-knative/knative-serving-operator:v0.6.0` or any other valid
operator image in the file config/operator.yaml, and run the following command:

```
kubectl apply -f config/
```

Please refer to [Building the Operator Image](#building-the-operator-image) to
build your own image.

To be clear, the operator will be deployed in the `default` namespace, and then
it will install Knative Serving in the `knative-serving` namespace.

## Prerequisites

### Istio

On OpenShift, Istio will get installed automatically if not already present by
using the [Maistra Operator](https://maistra.io/).

For other platforms, version 0.6.x of Knative Serving requires Istio CRD's:

```
kubectl apply -f https://raw.githubusercontent.com/knative/serving/v0.6.0/third_party/istio-1.0.7/istio-crds.yaml
```

### Operator SDK

This operator was created using the
[operator-sdk](https://github.com/operator-framework/operator-sdk/). It's not
strictly required but does provide some handy tooling.

## The `KnativeServing` Custom Resource

The installation of Knative Serving is triggered by the creation of
[a `KnativeServing` custom resource](config/crds/serving_v1alpha1_knativeserving_cr.yaml).
When it starts, the operator will _automatically_ create one of these in the
`knative-serving` namespace if it doesn't already exist.

The operator will ignore all other `KnativeServing` resources. Only the one
named `knative-serving` in the `knative-serving` namespace will trigger the
installation, reconfiguration, or removal of the knative serving resources.

The optional `spec.config` field can be used to set the corresponding entries in
the Knative Serving ConfigMaps. Conditions for a successful install and
available deployments will be updated in the `status` field, as well as which
version of Knative Serving the operator installed.

The following are all equivalent:

```
kubectl get knativeservings.serving.knative.dev -oyaml
kubectl get knativeserving -oyaml
kubectl get ks -oyaml
```

To uninstall Knative Serving, simply delete the `KnativeServing` resource.

```
kubectl delete ks -n knative-serving --all
```

## Development

It can be convenient to run the operator outside of the cluster to test changes.
The following command will build the operator and use your current "kube config"
to connect to the cluster:

```
operator-sdk up local --namespace=""
```

Pass `--help` for further details on the various `operator-sdk` subcommands, and
pass `--help` to the operator itself to see its available options:

```
operator-sdk up local --operator-flags "--help"
```

### Testing

To run end-to-end tests against your cluster:

```
operator-sdk test local ./test/e2e --namespace default
```

The `--namespace` parameter must match that of the `ServiceAccount` subject in
the [role_binding.yaml](config/role_binding.yaml).

## Building the Operator Image

To build the operator with `ko`, configure your an environment variable
`KO_DOCKER_REPO` as the docker repository to which developer images should be
pushed (e.g. `gcr.io/[gcloud-project]`, `docker.io/[username]`,
`quay.io/[repo-name]`, etc).

Install `ko` with the following command, if it is not available on your machine:

```
go get -u github.com/google/ko/cmd/ko
```

Then, build the operator image:

```
ko publish github.com/knative/serving-operator/cmd/manager -t $VERSION
```

You need to access the image by the name
`KO_DOCKER_REPO/manager-[md5]:$VERSION`, which you are able to find in the
output of the above `ko publish` command.

The image should match what's in [config/operator.yaml](config/operator.yaml)
and the `$VERSION` should match [version.go](version/version.go) and correspond
to the contents of [config/resources](config/resources/).

## Operator Framework

The remaining sections only apply if you wish to create the metadata required by
the
[Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager)

### Create a ClusterServiceVersion

The OLM requires special manifests that the operator-sdk can help generate.

Create a `ClusterServiceVersion` for the version that corresponds to the
manifest[s] beneath [config/resources](config/resources/). The
`$PREVIOUS_VERSION` is the CSV yours will replace.

```
operator-sdk olm-catalog gen-csv \
    --csv-version $VERSION \
    --from-version $PREVIOUS_VERSION \
    --update-crds
```

Most values should carry over, but if you're starting from scratch, some
post-editing of the file it generates may be required:

- Add fields to address any warnings it reports
- Verify `description` and `displayName` fields for all owned CRD's

### Create a CatalogSource

The [catalog.sh](hack/catalog.sh) script should yield a valid `ConfigMap` and
`CatalogSource` comprised of the `ClusterServiceVersions`,
`CustomResourceDefinitions`, and package manifest in the bundle beneath
[config/olm-catalog](config/olm-catalog/). You should apply its output in the
OLM namespace:

```
OLM_NS=$(kubectl get deploy --all-namespaces | grep olm-operator | awk '{print $1}')
./hack/catalog.sh | kubectl apply -n $OLM_NS -f -
```

### Using OLM on Minikube

You can test the operator using
[minikube](https://kubernetes.io/docs/setup/minikube/) after installing OLM on
it:

```
minikube start
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.0/crds.yaml
kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.10.0/olm.yaml
```

Once all the pods in the `olm` namespace are running, install the operator like
so:

```
./hack/catalog.sh | kubectl apply -n $OLM_NS -f -
```

Interacting with OLM is possible using `kubectl` but the OKD console is
"friendlier". If you have docker installed, use
[this script](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/scripts/run_console_local.sh)
to fire it up on <http://localhost:9000>.

#### Using kubectl

To install Knative Serving into the `knative-serving` namespace, simply
subscribe to the operator by running this script:

```
OLM_NS=$(kubectl get og --all-namespaces | grep olm-operators | awk '{print $1}')
OPERATOR_NS=$(kubectl get og --all-namespaces | grep global-operators | awk '{print $1}')
cat <<-EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: knative-serving-operator-sub
  generateName: knative-serving-operator-
  namespace: $OPERATOR_NS
spec:
  source: knative-serving-operator
  sourceNamespace: $OLM_NS
  name: knative-serving-operator
  channel: alpha
EOF
```
