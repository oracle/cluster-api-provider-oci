/*
 *
 * Copyright (c) 2022, Oracle and/or its affiliates.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 *
 */

package v1beta1

// validOcid is a simple pre-flight
// we will let the serverside handle the more complex and compete validation
func validOcid(ocid string) bool {
	if len(ocid) >= 4 && ocid[:4] == "ocid" {
		return true
	}

	return false
}

// validShape is a simple pre-flight
// we will let the serverside handle the more complex and compete validation
func validShape(shape string) bool {
	return len(shape) > 0
}
