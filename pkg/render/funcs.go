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

package render

// Functions available for all templates

// getOr returns the value of m[key] if it exists, fallback otherwise.
// As a special case, it also returns fallback if the value of m[key] is
// the empty string
func getOr(m map[string]interface{}, key, fallback string) interface{} {
	val, ok := m[key]
	if !ok {
		return fallback
	}

	s, ok := val.(string)
	if ok && s == "" {
		return fallback
	}

	return val
}

// isSet returns the value of m[key] if key exists, otherwise false
// Different from getOr because it will return zero values.
func isSet(m map[string]interface{}, key string) interface{} {
	val, ok := m[key]
	if !ok {
		return false
	}
	return val
}
