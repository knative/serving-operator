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
# Operator built from source.  It is started by Travis CI for each
# PR. For convenience, it can also be executed manually.

source $(dirname $0)/e2e-common.sh

knative_setup

# Let's see what the operator did
kubectl get pod --all-namespaces
kubectl logs -n knative-serving deployment/knative-serving-operator

# If we got this far, the operator installed Knative Serving
success
