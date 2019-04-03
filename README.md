# knative-serving-operator

If you don't already have it, install the latest
[operator-sdk](https://github.com/operator-framework/operator-sdk/)

Before doing anything else, grab your dependencies:

    $ dep ensure -v

## Run the Operator

The operator watches for an 'Install' custom resource, so you'll need
to register it:

    $ kubectl apply -f deploy/crds/serving_v1alpha1_install_crd.yaml

Once the operator is up (see next sections), trigger the installation
by creating an 'Install' CR. There are currently no fields expected in
its 'spec' but its 'status' should contain a list of the installed
resources and their version.

    $ kubectl apply -f deploy/crds/serving_v1alpha1_install_cr.yaml
    $ kubectl get install -oyaml

### Outside Cluster

    $ operator-sdk up local

To see the flags supported by the operator,

    $ operator-sdk up local --operator-flags "--help"

### Inside Cluster

We give the operator's service account cluster-admin privileges in the
default namespace.

    $ kubectl apply -f deploy/

## Create a Release

Update [the resource files](deploy/resources/) with the proper
[quay.io](https://quay.io/organization/openshift-knative) images, and
then run these commands:

    $ VERSION="vX.Y.Z"
    $ operator-sdk build --docker-build-args "--build-arg version=$VERSION" quay.io/openshift-knative/knative-serving-operator:$VERSION
    $ docker push quay.io/openshift-knative/knative-serving-operator:$VERSION
    $ git tag $VERSION
    $ git push --tags
