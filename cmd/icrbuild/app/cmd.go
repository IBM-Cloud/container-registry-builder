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

// Package app ...
package app

import (
	"fmt"
	"io"
	"os"

	"github.com/IBM-Cloud/container-registry-builder/pkg/icrbuild"
	"github.com/IBM-Cloud/container-registry-builder/pkg/icrbuild/version"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const logLevel = "debug"

// NewCommand creates the icrbuild command
func NewCommand(in io.Reader, out io.Writer, err io.Writer) *cobra.Command {
	options := icrbuild.NewBuildOptions(in, out, err)
	cmd := &cobra.Command{
		Use:   "icrbuild [DIRECTORY]",
		Short: "Build a Docker image in IBM Cloud Container Registry using builder contract",
		Args:  cobra.ExactArgs(1),
		Long: `
 `,
		Run: func(cmd *cobra.Command, args []string) {
			err := options.Run(cmd, args)
			if err != nil {
				logrus.Error(err)
			}
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := setUpLogs(err); err != nil {
				return err
			}
			cmd.SilenceUsage = true
			logrus.Infof("icrbuild %+v", version.Get())
			return nil
		},
	}

	cmd.Version = fmt.Sprintf("%+v", version.Get())
	cmd.SetVersionTemplate("{{printf .Version}}\n")

	//	[--no-cache] [--pull] [--quiet | -q] [--build-arg KEY=VALUE ...] [--file FILE | -f FILE] --tag TAG DIRECTORY
	cmd.PersistentFlags().BoolVar(&options.Flags.NoCache, "no-cache", false, "Optional: If specified, cached image layers from previous builds are not used in this build.")
	cmd.PersistentFlags().BoolVar(&options.Flags.Pull, "pull", false, "Optional: If specified, the base images are pulled even if an image with a matching tag already exists on the build host.")
	cmd.PersistentFlags().BoolVarP(&options.Flags.Quiet, "quiet", "q", false, "Optional: If specified, the build output is suppressed unless an error occurs.")
	cmd.PersistentFlags().StringArrayVar(&options.Flags.BuildArgs, "build-arg", nil, "Optional: Specify an additional build argument in the format 'KEY=VALUE'. The value of each build argument is available as an environment variable when you specify an ARG line that matches the key in your Dockerfile.")
	cmd.PersistentFlags().StringVarP(&options.Flags.File, "file", "f", "", "Optional: Specify the location of the Dockerfile relative to the build context. If not specified, the default is 'PATH/Dockerfile', where PATH is the root of the build context.")
	cmd.PersistentFlags().StringVarP(&options.Flags.Tag, "tag", "t", "", "The full name for the image that you want to build, which includes the registry URL and namespace.")
	cmd.MarkFlagRequired("tag")

	return cmd
}

func setUpLogs(out io.Writer) error {
	logrus.SetOutput(out)
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return err
	}
	logrus.SetLevel(lvl)
	return nil
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

// Run runs the command
func Run() error {
	cmd := NewCommand(os.Stdin, os.Stdout, os.Stderr)
	return cmd.Execute()
}
