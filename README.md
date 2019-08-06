# Knative Serving Operator

Knative Serving Operator is a project aiming to deploy and manage Knative
Serving in an automated way.

The following steps will install
[Knative Serving](https://github.com/knative/serving) and configure it
appropriately for your cluster in the `knative-serving` namespace. Please make
sure the [prerequisites](#Prerequisites) are installed first.

1. Install the
   [KnativeServing CRD](config/crds/serving_v1alpha1_knativeserving_crd.yaml)

   ```
   kubectl apply -f config/crds/serving_v1alpha1_knativeserving_crd.yaml
   ```

2. Install the operator

   To install from source code, run the command:

   ```
   ko apply -f config/
   ```

   To install from an existing image, change the value of `image` into
   `quay.io/openshift-knative/knative-serving-operator:v0.6.0` or any other
   valid operator image in the file config/operator.yaml, and run the following
   command:

   ```
   kubectl apply -f config/
   ```

3. Install the
   [KnativeServing custom resource](#the-knativeserving-custom-resource)

   ```
   kubectl apply -f config/crds/serving_v1alpha1_knativeserving_cr.yaml
   ```

Please refer to [Building the Operator Image](#building-the-operator-image) to
build your own image.

## Prerequisites

### Istio

On OpenShift, Istio will get installed automatically if not already present by
using the [Maistra Operator](https://maistra.io/).

For other platforms, version 0.6.x of Knative Serving requires Istio CRD's:

```
kubectl apply -f https://raw.githubusercontent.com/knative/serving/v0.6.0/third_party/istio-1.0.7/istio-crds.yaml
```

### Operator SDK

This operator was originally created using the
[operator-sdk](https://github.com/operator-framework/operator-sdk/). It's not
strictly required but does provide some handy tooling.

## The `KnativeServing` Custom Resource

The installation of Knative Serving is triggered by the creation of [a
`KnativeServing` custom resource
(CR)](config/crds/serving_v1alpha1_knativeserving_cr.yaml). The
operator will deploy Knative Serving in the same namespace containing
the `KnativeServing` CR, and this CR will trigger the installation,
reconfiguration, or removal of the knative serving resources.

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
kubectl delete ks --all
```

## Development

It can be convenient to run the operator outside of the cluster to test changes.
The following command will build the operator and use your current "kube config"
to connect to the cluster:

```
./hack/run-local.sh
```

Pass `--help` for further details on the various subcommands

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
ko publish knative.dev/serving-operator/cmd/manager -t $VERSION
```

You need to access the image by the name
`KO_DOCKER_REPO/manager-[md5]:$VERSION`, which you are able to find in the
output of the above `ko publish` command.

The image should match what's in [config/operator.yaml](config/operator.yaml)
and the `$VERSION` should match [version.go](version/version.go) and correspond
to the contents of [config/resources](config/resources/).
