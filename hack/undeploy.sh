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


set -eo pipefail

source "$(dirname $0)/common"

repo_dir="$(dirname $0)/.."
namespace=${1:-}
if [ -n "${namespace}" ] ; then
  namespace="-n ${namespace}"
fi

pushd ${repo_dir}/deploy
files="operator.yaml sriovoperatorconfig.yaml service_account.yaml role.yaml role_binding.yaml clusterrole.yaml clusterrolebinding.yaml configmap.yaml"
for file in ${files}; do
  envsubst< ${file} | ${OPERATOR_EXEC} delete --ignore-not-found ${namespace} -f -
done
${OPERATOR_EXEC} delete cm --ignore-not-found ${namespace} device-plugin-config
${OPERATOR_EXEC} delete MutatingWebhookConfiguration --ignore-not-found ${namespace} network-resources-injector-config sriov-operator-webhook-config
${OPERATOR_EXEC} delete ValidatingWebhookConfiguration --ignore-not-found ${namespace} sriov-operator-webhook-config
popd
