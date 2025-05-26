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


EXCLUSIONS=(operator.yaml) $(dirname $0)/deploy-setup.sh ${NAMESPACE}
source hack/env.sh
unset GOFLAGS
operator-sdk test local ./test/e2e --namespace ${NAMESPACE} --go-test-flags "-v" --up-local
# go test ./test/e2e/... -root=$(pwd) -kubeconfig=$KUBECONFIG -globalMan deploy/crds/sriovnetwork_v1_sriovnetwork_crd.yaml -localOperator -v -singleNamespace
hack/undeploy.sh sriov-network-operator
