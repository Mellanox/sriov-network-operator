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


cat <<'EOF' > /host/etc/udev/disable-nm-sriov.sh
#!/bin/bash
if [ ! -d "/sys/class/net/$1/device/physfn" ]; then
    exit 0
fi

pf_path=$(readlink /sys/class/net/$1/device/physfn -n)
pf_pci_address=${pf_path##*../}

if [ "$2" == "$pf_pci_address" ]; then
    echo "NM_UNMANAGED=1"
fi
EOF

chmod +x /host/etc/udev/disable-nm-sriov.sh
