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


set -eux

REPO=github.com/k8snetworkplumbingwg/sriov-network-operator
WHAT=${WHAT:-sriov-network-operator}
GOFLAGS=${GOFLAGS:-}
GLDFLAGS=${GLDFLAGS:-}

# eval $(go env | grep -e "GOHOSTOS" -e "GOHOSTARCH")

# : "${GOOS:=${GOHOSTOS}}"
# : "${GOARCH:=${GOHOSTARCH}}"
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)

# Go to the root of the repo
cdup="$(git rev-parse --show-cdup)" && test -n "$cdup" && cd "$cdup"

if [ -z ${VERSION_OVERRIDE+a} ]; then
	echo "Using version from git..."
	VERSION_OVERRIDE=$(git describe --abbrev=8 --dirty --always)
fi

GLDFLAGS+="-X ${REPO}/pkg/version.Raw=${VERSION_OVERRIDE}"

# eval $(go env)

if [ -z ${BIN_PATH+a} ]; then
	export BIN_PATH=build/_output/${GOOS}/${GOARCH}
fi

mkdir -p ${BIN_PATH}

CGO_ENABLED=${CGO_ENABLED:-0}


if [[ ${WHAT} == "manager" ]]; then
echo "Building ${WHAT} (${VERSION_OVERRIDE})"
CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS} -s -w" -o ${BIN_PATH}/${WHAT} main.go
else
echo "Building ${REPO}/cmd/${WHAT} (${VERSION_OVERRIDE})"
CGO_ENABLED=${CGO_ENABLED} GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS} -s -w" -o ${BIN_PATH}/${WHAT} ${REPO}/cmd/${WHAT}
fi
