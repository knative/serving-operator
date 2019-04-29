#!/usr/bin/env bash

set -ex

pushd $(dirname "$0")/..

IMAGE=$(basename $(pwd))

eval VERSION=v$(grep Version version/version.go | awk '{print $3}')
operator-sdk build quay.io/openshift-knative/$IMAGE:$VERSION
docker push quay.io/openshift-knative/$IMAGE:$VERSION
git tag -f $VERSION
git push --tags --force

popd
