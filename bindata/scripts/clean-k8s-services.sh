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


if [ "$CLUSTER_TYPE" == "openshift" ]; then
  echo "openshift cluster"
  exit
fi

chroot_path="/host"


ovs_service=$chroot_path/usr/lib/systemd/system/ovs-vswitchd.service
ovs_dropin_dir=$chroot_path/usr/lib/systemd/system/ovs-vswitchd.service.d
ovs_dropin=$ovs_dropin_dir/10-hw-offload.conf

needs_reload=""

if [ -f $ovs_dropin ]; then
  rm -f $ovs_dropin
  rmdir $ovs_dropin_dir >/dev/null 2>&1 || true
  needs_reload="yes"
fi

# Older versions injected hw-offload directly into the service file instead of a drop-in
if [ -f $ovs_service ]; then
  if grep -q hw-offload $ovs_service; then
    sed -i.bak '/hw-offload/d' $ovs_service
    needs_reload="yes"
  fi
fi

if [ -n "$needs_reload" ]; then
  chroot $chroot_path /bin/bash -c systemctl daemon-reload >/dev/null 2>&1 || true
fi
