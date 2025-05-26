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

# This script inits a cluster to allow sriov-network-operator
# to deploy.  It assumes it is capable of login as a
# user who has the cluster-admin role

set -eo pipefail

source "$(dirname $0)/common"

load_manifest() {
  local repo=$1
  local namespace=${2:-}
  export NAMESPACE=${namespace}
  if [ -n "${namespace}" ] ; then
    namespace="-n ${namespace}"
  fi

  pushd ${repo}/deploy
    if ! ${OPERATOR_EXEC} get ns $2 > /dev/null 2>&1 && test -f namespace.yaml ; then
      
      envsubst< namespace.yaml | ${OPERATOR_EXEC} apply -f -
    fi
    files="service_account.yaml role.yaml role_binding.yaml clusterrole.yaml clusterrolebinding.yaml configmap.yaml sriovoperatorconfig.yaml operator.yaml"
    for m in ${files}; do
      if [ "$(echo "${EXCLUSIONS[@]}" | grep -o ${m} | wc -w | xargs)" == "0" ] ; then
        envsubst< ${m} | ${OPERATOR_EXEC} apply ${namespace:-} --validate=false -f -
      fi
    done

  popd
}

# This is required for when running the operator locally using go run
rm -rf /tmp/_working_dir
mkdir /tmp/_working_dir
source hack/env.sh

load_manifest ${repo_dir} $1
