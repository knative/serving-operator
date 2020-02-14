#!/usr/bin/env bash

# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs the end-to-end tests against Knative Serving
# Operator built from source. However, this script will download the latest
# source code of knative serving and generated the latest manifest file for
# serving installation. The serving operator will use this newly generated
# manifest file to replace the one under the directory
# cmd/manager/kodata/knative-serving. This purpose of this script is to verify
# whether the latest source code of operator can work properly with the latest
# source code of knative serving.

# If you already have a Knative cluster setup and kubectl pointing
# to it, call this script with the --run-tests arguments and it will use
# the cluster and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start knative in it, run the tests and delete the
# cluster.

source $(dirname $0)/e2e-common.sh

function install_latest_operator_release() {
  header "Installing Knative Serving operator latest public release"
  local full_url="https://github.com/knative/serving-operator/releases/download/${LATEST_SERVING_OPERATOR_RELEASE_VERSION}/serving-operator.yaml"

  local release_yaml="$(mktemp)"
  wget "${full_url}" -O "${release_yaml}" \
      || fail_test "Unable to download latest Knative Serving Operator release."
  download_serving
  install_istio || fail_test "Istio installation failed"
  kubectl apply -f "${release_yaml}" || fail_test "Knative Serving Operator latest release installation failed"
}

function create_custom_resource() {
  echo ">> Creating the custom resource of Knative Serving:"
  cat <<EOF | kubectl apply -f -
apiVersion: operator.knative.dev/v1alpha1
kind: KnativeServing
metadata:
  name: knative-serving
  namespace: ${TEST_NAMESPACE}
spec:
  config:
    defaults:
      revision-timeout-seconds: "300"  # 5 minutes
    autoscaler:
      stable-window: "60s"
    deployment:
      registriesSkippingTagResolving: "ko.local,dev.local"
    logging:
      loglevel.controller: "debug"
EOF
}

function knative_setup() {
  create_namespace
  install_latest_operator_release
  create_custom_resource
  wait_until_pods_running_label ${TEST_NAMESPACE} "-l serving.knative.dev/release=${LATEST_SERVING_RELEASE_VERSION}"
  generate_latest_serving_manifest
}

# Create test resources and images
function test_setup() {
  echo ">> Creating test resources (test/configuration/)"
  #cd ${KNATIVE_SERVING_DIR}/serving
  ko apply ${KO_FLAGS} -f test/configuration/ || return 1
  if (( MESH )); then
    kubectl label namespace serving-tests istio-injection=enabled
    kubectl label namespace serving-tests-alt istio-injection=enabled
    ko apply ${KO_FLAGS} -f test/configuration/mtls/ || return 1
  fi

  echo ">> Uploading test images for upgrade"
  ${OPERATOR_DIR}/test/upload-test-images.sh

  echo ">> Waiting for Ingress provider to be running..."
  if [[ -n "${ISTIO_VERSION}" ]]; then
    wait_until_pods_running istio-system || return 1
    wait_until_service_has_external_ip istio-system istio-ingressgateway
  fi

  # Go back to the directory of operator
  cd ${OPERATOR_DIR}
}

function generate_latest_serving_manifest() {
  cd ${KNATIVE_SERVING_DIR}/serving
  COMMIT_ID=$(git rev-parse --verify HEAD)
  echo ">> The latest commit ID of Knative Serving is ${COMMIT_ID}."
  mkdir -p output

  # Generate the manifest
  export YAML_OUTPUT_DIR=${KNATIVE_SERVING_DIR}/serving/output
  ./hack/generate-yamls.sh ${KNATIVE_SERVING_DIR}/serving ${YAML_OUTPUT_DIR}/output.yaml

  # Copy the serving.yaml into cmd/manager/kodata/knative-serving
  SERVING_YAML=${KNATIVE_SERVING_DIR}/serving/output/serving.yaml
  if [[ -f "${SERVING_YAML}" ]]; then
    echo ">> Replacing the current manifest in operator with the generated manifest"
    rm -rf ${OPERATOR_DIR}/cmd/manager/kodata/knative-serving/*
    cp ${SERVING_YAML} ${OPERATOR_DIR}/cmd/manager/kodata/knative-serving/serving.yaml
  else
    echo ">> The serving.yaml was not generated, so keep the current manifest"
  fi

  # Go back to the directory of operator
  cd ${OPERATOR_DIR}
}

# Skip installing istio as an add-on
initialize $@ --skip-istio-addon

TIMEOUT=20m

header "Running preupgrade tests under serving repo"

#cd ${KNATIVE_SERVING_DIR}/serving
go_test_e2e -tags=preupgrade -timeout=${TIMEOUT} ./test/upgrade \
  --resolvabledomain="false" "--https" || fail_test

# Remove this in case we failed to clean it up in an earlier test.
rm -f /tmp/prober-signal

cd ${OPERATOR_DIR}
go_test_e2e -tags=probe -timeout=${TIMEOUT} ./test/upgrade \
  --resolvabledomain="false" "--https" &
PROBER_PID=$!
echo "Prober PID is ${PROBER_PID}"

install_serving_operator_head
wait_until_pods_running_label ${TEST_NAMESPACE} "-l serving.knative.dev/release=v20200218-a8140d5d6"

# If we got this far, the operator installed Knative Serving of the latest source code.
header "Running tests for Knative Serving Operator"
failed=0

# Run the postupgrade tests under the operator repo
go_test_e2e -tags=postupgrade -timeout=${TIMEOUT} ./test/upgrade || failed=1

header "Running tests for Knative Serving"
# Run the postupgrade tests under the serving repo
#cd ${KNATIVE_SERVING_DIR}/serving
go_test_e2e -tags=postupgrade1 -timeout=${TIMEOUT} ./test/upgrade || failed=1

cd ${OPERATOR_DIR}
install_latest_operator_release
wait_until_pods_running_label ${TEST_NAMESPACE} "-l serving.knative.dev/release=${LATEST_SERVING_RELEASE_VERSION}"

header "Running postdowngrade tests"
#cd ${KNATIVE_SERVING_DIR}/serving
go_test_e2e -tags=postdowngrade -timeout=${TIMEOUT} ./test/upgrade \
  --resolvabledomain="false" || fail_test

echo "done" > /tmp/prober-signal

cd ${OPERATOR_DIR}
header "Waiting for prober test"
wait ${PROBER_PID} || fail_test "Prober failed"

# Require that tests succeeded.
(( failed )) && fail_test

success
