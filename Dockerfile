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

FROM golang:1.23 AS builder
WORKDIR /go/src/github.com/k8snetworkplumbingwg/sriov-network-operator
COPY . .
RUN make _build-manager BIN_PATH=build/_output/cmd
RUN make _build-sriov-network-operator-config-cleanup BIN_PATH=build/_output/cmd

FROM quay.io/centos/centos:stream9
COPY --from=builder /go/src/github.com/k8snetworkplumbingwg/sriov-network-operator/build/_output/cmd/manager /usr/bin/sriov-network-operator
COPY --from=builder /go/src/github.com/k8snetworkplumbingwg/sriov-network-operator/build/_output/cmd/sriov-network-operator-config-cleanup /usr/bin/sriov-network-operator-config-cleanup
COPY bindata /bindata
ENV OPERATOR_NAME=sriov-network-operator
CMD ["/usr/bin/sriov-network-operator"]
