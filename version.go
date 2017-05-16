/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kops

import "strings"

// This should be replaced by the makefile
var Version = "1.5.0"

// This should be replaced by the makefile
var GitVersion = ""

// DefaultProtokubeImageName is the name of the protokube image, as we would pass to "docker run"
func DefaultDnsControllerImageName() string {
	// + is valid in semver, but not in docker tags.
	// Note that this mirrors the logic in the makefile for PROTOKUBE_TAG.
	dockerTag := strings.Replace(Version, "+", "-", -1)
	return "kope/dns-controller:" + dockerTag
}

// DefaultProtokubeImageName is the name of the protokube image, as we would pass to "docker run"
func DefaultProtokubeImageName() string {
	// + is valid in semver, but not in docker tags.
	// Note that this mirrors the logic in the makefile for PROTOKUBE_TAG.
	dockerTag := strings.Replace(Version, "+", "-", -1)
	return "protokube:" + dockerTag
}
