// Copyright 2025 sriov-network-device-plugin authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package featuregate

import (
	"fmt"
	"strings"
	"sync"
)

// FeatureGate provides methods to check state of the feature
type FeatureGate interface {
	// IsEnabled returns state of the feature,
	// if feature name is unknown will always return false
	IsEnabled(feature string) bool
	// Init set state for the features from the provided map.
	// completely removes the previous state
	Init(features map[string]bool)
	// String returns string representation of the feature state
	String() string
}

// New returns default implementation of the FeatureGate interface
func New() FeatureGate {
	return &featureGate{
		lock:  &sync.RWMutex{},
		state: map[string]bool{},
	}
}

type featureGate struct {
	lock  *sync.RWMutex
	state map[string]bool
}

// IsEnabled returns state of the feature,
// if feature name is unknown will always return false
func (fg *featureGate) IsEnabled(feature string) bool {
	fg.lock.RLock()
	defer fg.lock.RUnlock()
	return fg.state[feature]
}

// Init set state for the features from the provided map.
// completely removes the previous state
func (fg *featureGate) Init(features map[string]bool) {
	fg.lock.Lock()
	defer fg.lock.Unlock()
	fg.state = make(map[string]bool, len(features))
	for k, v := range features {
		fg.state[k] = v
	}
}

// String returns string representation of the feature state
func (fg *featureGate) String() string {
	fg.lock.RLock()
	defer fg.lock.RUnlock()
	var result strings.Builder
	var sep string
	for k, v := range fg.state {
		result.WriteString(fmt.Sprintf("%s%s:%t", sep, k, v))
		sep = ", "
	}
	return result.String()
}
