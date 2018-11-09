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

// Package icrbuild ...
package icrbuild

import (
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/image"
	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// BuildFlags are the flags for the docker build
type BuildFlags struct {
	NoCache   bool
	Pull      bool
	Quiet     bool
	BuildArgs []string
	File      string
	Tag       string
}

// BuildOptions hold the io streams for the build
type BuildOptions struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	Flags BuildFlags
}

// BuildRunner holds the the method that runs the build
type BuildRunner interface {
	Run(cmd *cobra.Command, args []string) error
}

// NewBuildOptions hold the streams for the build
func NewBuildOptions(in io.Reader, out io.Writer, err io.Writer) *BuildOptions {
	return &BuildOptions{
		In:  in,
		Out: out,
		Err: err,
	}
}

// Run the method that runs the build
func (o *BuildOptions) Run(cmd *cobra.Command, args []string) error {

	var (
		registryClient          *IBMRegistrySession
		imageName, buildContext string
		err                     error
		cli                     *builderCLI
		ccmd                    *cobra.Command
	)

	if !reference.ReferenceRegexp.MatchString(o.Flags.Tag) {
		return errors.Errorf("Image Name is not correct format!")
	}

	registryClient, imageName, err = NewRegistryClient(o.Flags.Tag)
	if err != nil {
		return errors.Wrap(err, "Unable to Connect to IBM Cloud")
	}

	logrus.Debugf("Running IBM Container Registry build: context: %s, dockerfile: %s", args[0], o.Flags.File)

	buildContext, err = filepath.Abs(args[0])
	if err != nil {
		logrus.Errorf("Error parsing build context: %v", err)
		return errors.Wrap(err, "Docker build Context error! Check supplied context path")
	}

	cli = &builderCLI{*command.NewDockerCli(os.Stdin, os.Stdout, os.Stderr, false), NewBuilder(registryClient)}

	ccmd = image.NewBuildCommand(cli)

	ccmd.Flags().Set("tag", imageName)
	ccmd.Flags().Set("no-cache", strconv.FormatBool(o.Flags.NoCache))
	ccmd.Flags().Set("quiet", strconv.FormatBool(o.Flags.Quiet))
	ccmd.Flags().Set("pull", strconv.FormatBool(o.Flags.Pull))
	ccmd.Flags().Set("disable-content-trust", "true")
	ccmd.Flags().Set("file", o.Flags.File)
	for _, buildFlag := range o.Flags.BuildArgs {
		ccmd.Flags().Set("build-arg", buildFlag)
	}

	// Woraround a defect whem term is set
	os.Unsetenv("TERM")
	err = ccmd.RunE(nil, []string{buildContext})

	return err
}
