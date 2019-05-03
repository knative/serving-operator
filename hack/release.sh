#!/usr/bin/env bash

set -ex

pushd $(dirname "$0")/..

readonly HUB_ORG="${HUB_ORG:-"openshift-knative"}"

IMAGE=$(basename $(pwd))

eval VERSION=v$(grep Version version/version.go | awk '{print $3}')
operator-sdk build quay.io/$HUB_ORG/$IMAGE:$VERSION
docker push quay.io/$HUB_ORG/$IMAGE:$VERSION
git tag -f $VERSION
git push --tags --force

popd
