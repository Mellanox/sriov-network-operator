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

# Teardown KinD cluster

if ! command -v kind &> /dev/null; then
  echo "KinD is not available"
  exit 1
fi

if systemctl is-active vf-switcher.service -q;then
    sudo systemctl stop vf-switcher.service
fi
sudo rm -f /etc/vf-switcher/vf-switcher.yaml
kind delete cluster

