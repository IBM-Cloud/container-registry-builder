// ------------------------------------------------------------------------------
// Copyright IBM Corp. 2018
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ------------------------------------------------------------------------------

// Package version ...
package version

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
)

var version, gitCommit, gitTreeState, buildDate string
var platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

// Info struct holds the version info
type Info struct {
	Version      string
	GitVersion   string
	GitCommit    string
	GitTreeState string
	BuildDate    string
	GoVersion    string
	Compiler     string
	Platform     string
}

// Get returns the version and buildtime information about the binary
func Get() *Info {
	// These variables typically come from -ldflags settings to `go build`
	return &Info{
		Version:      version,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     platform,
	}
}

// ParseVersion parses the version string
func ParseVersion(version string) (semver.Version, error) {
	version = strings.TrimSpace(version)
	v, err := semver.Parse(strings.TrimLeft(version, "v"))
	if err != nil {
		return semver.Version{}, errors.Wrap(err, "can't parse semver")
	}
	return v, nil
}
