# Knative Serving Operator

The following will install [Knative
Serving](https://github.com/knative/serving) and configure it
appropriately for your cluster in the `default` namespace:

    kubectl apply -f deploy/crds/serving_v1alpha1_install_crd.yaml
    kubectl apply -f deploy/
    kubectl apply -f deploy/crds/serving_v1alpha1_install_cr.yaml

## Prerequisites

### Istio

On OpenShift, Istio will get installed automatically if not already
present by using the [Maistra Operator](https://maistra.io/).

For other platforms, version 0.5.x of Knative Serving requires Istio CRD's:

    kubectl apply -f https://github.com/knative/serving/releases/download/v0.5.2/istio-crds.yaml

### Operator SDK

This operator was created using the
[operator-sdk](https://github.com/operator-framework/operator-sdk/).
It's not strictly required but does provide some handy tooling.

## The Install Custom Resource

The installation of Knative Serving is triggered by the creation of
[an `Install` custom
resource](deploy/crds/serving_v1alpha1_install_cr.yaml).

The optional `spec.config` field can be used to set the corresponding
entries in the Knative Serving ConfigMaps. Conditions for a successful
install and available deployments will be updated in the `status`
field, as well as which version of Knative Serving the operator
installed.

The following are all equivalent, but the latter may suffer from name
conflicts.

    kubectl get installs.serving.knative.dev -oyaml
    kubectl get ksi -oyaml
    kubectl get install -oyaml

To uninstall Knative Serving, simply delete the `Install` resource.

    kubectl delete ksi --all
    
## Development

It can be convenient to run the operator outside of the cluster to
test changes. The following command will build the operator and use
your current "kube config" to connect to the cluster:

    operator-sdk up local

Pass `--help` for further details on the various `operator-sdk`
subcommands, and pass `--help` to the operator itself to see its
available options:

    operator-sdk up local --operator-flags "--help"

### Testing

To run end-to-end tests against your cluster:

    operator-sdk test local ./test/e2e --namespace default

The `--namespace` parameter must match that of the `ServiceAccount`
subject in the [role_binding.yaml](deploy/role_binding.yaml).

### Building the Operator Image

To build the operator,

    operator-sdk build quay.io/$REPO/knative-serving-operator:$VERSION

The image should match what's in
[deploy/operator.yaml](deploy/operator.yaml) and the `$VERSION` should
match [version.go](version/version.go) and correspond to the contents
of [deploy/resources](deploy/resources/).

There is a handy script that will build and push an image to
[quay.io](https://quay.io/repository/openshift-knative/knative-serving-operator)
and tag the source:

    ./hack/release.sh

## Operator Framework

The remaining sections only apply if you wish to create the metadata
required by the [Operator Lifecycle
Manager](https://github.com/operator-framework/operator-lifecycle-manager)

### Create a ClusterServiceVersion

The OLM requires special manifests that the operator-sdk can help
generate.

Create a `ClusterServiceVersion` for the version that corresponds to
the manifest[s] beneath [deploy/resources](deploy/resources/). The
`$PREVIOUS_VERSION` is the CSV yours will replace.

    operator-sdk olm-catalog gen-csv \
        --csv-version $VERSION \
        --from-version $PREVIOUS_VERSION \
        --update-crds

Most values should carry over, but if you're starting from scratch,
some post-editing of the file it generates may be required:

* Add fields to address any warnings it reports
* Verify `description` and `displayName` fields for all owned CRD's

### Create a CatalogSource

The [catalog.sh](hack/catalog.sh) script should yield a valid
`ConfigMap` and `CatalogSource` comprised of the
`ClusterServiceVersions`, `CustomResourceDefinitions`, and package
manifest in the bundle beneath
[deploy/olm-catalog](deploy/olm-catalog/). You should apply its output
in the OLM namespace:

    OLM=$(kubectl get pods --all-namespaces | grep olm-operator | head -1 | awk '{print $1}')
    ./hack/catalog.sh | kubectl apply -n $OLM -f -

### Using OLM on Minikube

You can test the operator using
[minikube](https://kubernetes.io/docs/setup/minikube/) after
installing OLM on it:

    minikube start
    kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/0.9.0/olm.yaml

Once all the pods in the `olm` namespace are running, install the
operator like so:
    
    ./hack/catalog.sh | kubectl apply -n olm -f -

Interacting with OLM is possible using `kubectl` but the OKD console
is "friendlier". If you have docker installed, use [this
script](https://github.com/operator-framework/operator-lifecycle-manager/blob/master/scripts/run_console_local.sh)
to fire it up on <http://localhost:9000>.

#### Using kubectl

To install Knative Serving into the `knative-serving` namespace, apply
the following resources:

```
cat <<-EOF | kubectl apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: knative-serving
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: knative-serving
  namespace: knative-serving
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: knative-serving-operator-sub
  generateName: knative-serving-operator-
  namespace: knative-serving
spec:
  source: knative-serving-operator
  sourceNamespace: olm
  name: knative-serving-operator
  channel: alpha
---
apiVersion: serving.knative.dev/v1alpha1
kind: Install
metadata:
  name: knative-serving
  namespace: knative-serving
EOF
```
