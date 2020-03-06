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

# This script builds all the YAMLs that Knative serving operator publishes.
# It may be varied between different branches, of what it does, but the
# following usage must be observed:
#
# generate-yamls.sh  <repo-root-dir> <generated-yaml-list>
#     repo-root-dir         the root directory of the repository.
#     generated-yaml-list   an output file that will contain the list of all
#                           YAML files. The first file listed must be our
#                           manifest that contains all images to be tagged.

# Different versions of our scripts should be able to call this script with
# such assumption so that the test/publishing/tagging steps can evolve
# differently than how the YAMLs are built.

# The following environment variables affect the behavior of this script:
# * `$KO_FLAGS` Any extra flags that will be passed to ko.
# * `$YAML_OUTPUT_DIR` Where to put the generated YAML files, otherwise a
#   random temporary directory will be created. **All existing YAML files in
#   this directory will be deleted.**
# * `$KO_DOCKER_REPO` If not set, use ko.local as the registry.

set -o errexit
set -o pipefail
set -o xtrace

readonly YAML_REPO_ROOT=${1:?"First argument must be the repo root dir"}
readonly YAML_LIST_FILE=${2:?"Second argument must be the output file"}

# Set output directory
if [[ -z "${YAML_OUTPUT_DIR:-}" ]]; then
  readonly YAML_OUTPUT_DIR="${YAML_REPO_ROOT}/output"
  mkdir -p "${YAML_OUTPUT_DIR}"
fi
rm -fr ${YAML_OUTPUT_DIR}/*.yaml

# Generated Knative Operator component YAML files
readonly SERVING_OPERATOR_YAML=${YAML_OUTPUT_DIR}/serving-operator.yaml

# Flags for all ko commands
KO_YAML_FLAGS="-P"
[[ "${KO_DOCKER_REPO}" != gcr.io/* ]] && KO_YAML_FLAGS=""
readonly KO_YAML_FLAGS="${KO_YAML_FLAGS} ${KO_FLAGS} --strict"

: ${KO_DOCKER_REPO:="ko.local"}
export KO_DOCKER_REPO

cd "${YAML_REPO_ROOT}"

echo "Building Knative Serving Operator"
ko resolve ${KO_YAML_FLAGS} -f config/ > "${SERVING_OPERATOR_YAML}"

# List generated YAML files. We have only one serving-operator.yaml so far.

ls -1 ${SERVING_OPERATOR_YAML} > ${YAML_LIST_FILE}
# TODO(adrcunha): Uncomment once there's more than one YAML generated.
# ls -1 ${YAML_OUTPUT_DIR}/*.yaml | grep -v ${SERVING_OPERATOR_YAML} >> ${YAML_LIST_FILE}
