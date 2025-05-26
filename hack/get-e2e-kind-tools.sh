#!/usr/bin/env bash
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

set -o errexit
# ensure this file is sourced to add required components to PATH

here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
root="$(readlink --canonicalize "$here/..")"
VERSION="v0.10.0"
KIND_BINARY_URL="https://github.com/kubernetes-sigs/kind/releases/download/${VERSION}/kind-$(uname)-amd64"
K8_STABLE_RELEASE_URL="https://storage.googleapis.com/kubernetes-release/release/stable.txt"
JQ_RELEASE_URL="https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64"

if [ ! -d "${root}/bin" ]; then
      mkdir "${root}/bin"
fi

echo "retrieving kind"
curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo "${root}/bin/kind" "${KIND_BINARY_URL}"
chmod +x "${root}/bin/kind"

echo "retrieving kubectl"
curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo "${root}/bin/kubectl" "https://storage.googleapis.com/kubernetes-release/release/$(curl -s ${K8_STABLE_RELEASE_URL})/bin/linux/amd64/kubectl"
chmod +x "${root}/bin/kubectl"

echo "retrieving jq"
curl --max-time 10 --retry 10 --retry-delay 5 --retry-max-time 60 -Lo "${root}/bin/jq" "${JQ_RELEASE_URL}"
chmod +x "${root}/bin/jq"
export PATH="$PATH:$root/bin"
