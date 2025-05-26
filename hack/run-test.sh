#!/bin/bash
# Copyright 2025 sriov-network-device-plugin authors
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
#
# SPDX-License-Identifier: Apache-2.0

set -euxo pipefail

NAMESPACE=${NAMESPACE:-openshift-sriov-network-operator}
DIR=$PWD
GOFLAGS=${GOFLAGS:-}

export TEST_NAMESPACE=${NAMESPACE}
export KUBECONFIG=${KUBECONFIG:-/root/dev-scripts/ocp/sriov/auth/kubeconfig}


cd $DIR
# GO111MODULE=on go test ./test/operator/...  -root=$OPERATOR_ROOT -kubeconfig=$KUBECONFIG -globalMan $OPERATOR_ROOT/deploy/crds/sriovnetwork.openshift.io_sriovnetworks_crd.yaml -namespacedMan $OPERATOR_ROOT/deploy/operator-init.yaml -v -singleNamespace true
ginkgo -v --progress ./test/$1 -- -root=$DIR -kubeconfig=$KUBECONFIG -globalMan $DIR/hack/dummy.yaml -namespacedMan $DIR/hack/dummy.yaml
