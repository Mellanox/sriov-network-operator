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


here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
root="$(readlink --canonicalize "$here/..")"

GOPATH="${GOPATH:-$HOME/go}"
JUNIT_OUTPUT="${JUNIT_OUTPUT:-/tmp/artifacts}"
export PATH=$PATH:$GOPATH/bin

${root}/bin/ginkgo --timeout=3h -output-dir=$JUNIT_OUTPUT --junit-report "unit_report.xml" -v "$SUITE" -- -report=$JUNIT_OUTPUT
