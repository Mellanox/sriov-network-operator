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


ATTEMPTS=0
MAX_ATTEMPTS=72
ready=false
sleep_time=10

until $ready || [ $ATTEMPTS -eq $MAX_ATTEMPTS ]
do
    echo "running tests"
    if SUITE=./test/validation ./hack/run-e2e-conformance.sh; then
        echo "succeeded"
        ready=true
    else    
        echo "failed, retrying"
        sleep $sleep_time
    fi
    (( ATTEMPTS++ ))
done

if ! $ready; then 
    echo "Timed out waiting for features to be ready"
    ${OPERATOR_EXEC} get nodes
    ${OPERATOR_EXEC} cluster-info dump -n ${NAMESPACE}
    exit 1
fi
