# knative-serving-operator

If you don't already have it, install
[operator-sdk](https://github.com/operator-framework/operator-sdk/)

## Create a release

Update the files beneath [deploy/resources/] with the proper
[quay.io](https://quay.io/organization/openshift-knative) images, and
then run these commands:

    $ VERSION="vX.Y.Z"
    $ operator-sdk build --docker-build-args "--build-arg version=$VERSION" quay.io/openshift-knative/knative-serving-operator:$VERSION
    $ docker push quay.io/openshift-knative/knative-serving-operator:$VERSION

