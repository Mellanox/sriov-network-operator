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

source hack/env.sh
EXCLUSIONS=(operator.yaml) $(dirname $0)/deploy-setup.sh ${NAMESPACE}

export TEST_NAMESPACE=${NAMESPACE}
KUBECONFIG=${KUBECONFIG:-/root/env/ign/auth/kubeconfig}

echo ${SRIOV_CNI_IMAGE}
echo ${SRIOV_INFINIBAND_CNI_IMAGE}
echo ${OVS_CNI_IMAGE}
echo ${RDMA_CNI_IMAGE}
echo ${SRIOV_DEVICE_PLUGIN_IMAGE}
echo ${NETWORK_RESOURCES_INJECTOR_IMAGE}
echo ${SRIOV_NETWORK_CONFIG_DAEMON_IMAGE}
echo ${SRIOV_NETWORK_OPERATOR_IMAGE}
echo ${SRIOV_NETWORK_WEBHOOK_IMAGE}
echo ${METRICS_EXPORTER_IMAGE}
envsubst < deploy/operator.yaml  > deploy/operator-init.yaml
go test ./test/e2e/... -root=$(pwd) -kubeconfig=$KUBECONFIG -globalMan deploy/crds/sriovnetwork.openshift.io_sriovnetworks_crd.yaml -namespacedMan deploy/operator-init.yaml -v -singleNamespace true
