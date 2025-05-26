#!/bin/sh
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

if [ "$IS_CONTAINER" != "" ]; then
  for TARGET in "${@}"; do
    find "${TARGET}" -name '*.go' ! -path '*/vendor/*' ! -path '*/build/*' -exec gofmt -s -w {} \+
  done
  git diff --exit-code
else
  $CONTAINER_CMD run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/go/src/github.com/k8snetworkplumbingwg/sriov-network-operator:z" \
    --workdir /go/src/github.com/k8snetworkplumbingwg/sriov-network-operator \
    openshift/origin-release:golang-1.12 \
    ./hack/go-fmt.sh "${@}"
fi