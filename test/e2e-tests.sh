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

set -o xtrace

OPERATOR_DIR=$(dirname $0)/..
KNATIVE_SERVING_DIR=${OPERATOR_DIR}/..

function knative_setup() {
  install_istio || fail_test "Istio installation failed"
  generate_latest_serving_manifest
  install_serving_operator
}

function generate_latest_serving_manifest() {
  # Go the directory to download the source code of knative serving
  cd ${KNATIVE_SERVING_DIR}

  # Download the source code of knative serving
  git clone https://github.com/knative/serving.git
  cd serving
  git checkout 143c72e47c3076e9d392c7c90c60f33da649554a
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

# If we got this far, the operator installed Knative Serving of the latest source code.
header "Running tests for Knative Serving Operator"
failed=0

# Run the integration tests
go_test_e2e -timeout=20m ./test/e2e || failed=1

# Require that tests succeeded.
(( failed )) && fail_test

success
