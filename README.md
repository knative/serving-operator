# knative-serving-operator

If you don't already have it, install the latest
[operator-sdk](https://github.com/operator-framework/operator-sdk/)

Before doing anything else, grab your dependencies:

    $ dep ensure -v

Version 0.5.1 of knative-serving refers to Istio CRD's:

    $ kubectl apply -f https://github.com/knative/serving/releases/download/v0.5.1/istio-crds.yaml

## Run the Operator

The operator watches for an `Install` custom resource, so you'll need
to register it:

    $ kubectl apply -f deploy/crds/serving_v1alpha1_install_crd.yaml

Once the operator is up (see next sections), trigger the installation
by creating an `Install` CR. There are currently no fields expected in
its `spec` but its `status` should contain a list of the installed
resources and their version.

    $ kubectl apply -f deploy/crds/serving_v1alpha1_install_cr.yaml
    $ kubectl get install -oyaml

To uninstall,

    $ kubectl delete -f deploy/crds/serving_v1alpha1_install_cr.yaml
    
### Outside Cluster

    $ operator-sdk up local

To see the flags supported by the operator,

    $ operator-sdk up local --operator-flags "--help"

### Inside Cluster

We give the operator's service account `cluster-admin` privileges in
the default namespace.

    $ kubectl apply -f deploy/

## Create a Release

Verify that [version.go](version/version.go) matches the contents of
[deploy/resources](deploy/resources/) and then run the following to
build and push an image for the operator to
[quay.io](https://quay.io/repository/openshift-knative/knative-serving-operator).

    ./hack/release.sh

## Create a CatalogSource for [OLM](https://github.com/operator-framework/operator-lifecycle-manager)

The OLM requires special manifests that the operator-sdk can help
generate.

Create a `ClusterServiceVersion` for the version that corresponds to
those manifest[s] beneath [deploy/resources](deploy/resources/). The
`$PREVIOUS_VERSION` is the CSV yours will replace.

    operator-sdk olm-catalog gen-csv \
        --csv-version $VERSION \
        --from-version $PREVIOUS_VERSION

Most values should carry over, but if you're starting from scratch,
some post-editing of the file it generates will be required:

* Add fields to address any warnings it reports
* Verify `description` and `displayName` fields for all owned CRD's

The [catalog.sh](hack/catalog.sh) script should yield a valid
`CatalogSource` for you to publish.

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

