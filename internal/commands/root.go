// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package commands implements the ackctl commands.
package commands

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aws-controllers-k8s/ackctl/internal/debuglog"
)

var rootCmd = &cobra.Command{
	Use:   "ackctl",
	Short: "Command-line tools for AWS Controllers for Kubernetes (ACK)",
	Long: `ackctl is a command-line companion for AWS Controllers for Kubernetes.

Available features:

  adopt          Bring existing AWS resources under ACK management by tag,
                 emitting adoption Custom Resources so you do not have to
                 hand-write adoption-fields annotations.
  list adoptable Show which ACK resource kinds can be adopted by tag, the
                 identifiers each needs, and why the rest cannot.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	// Registered before cobra adds --version, which then forgoes the -v shorthand.
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		level, err := resolveVerbosity(cmd)
		if err != nil {
			return err
		}
		debuglog.SetLevel(level)
		return nil
	},
}

// verbosityEnv raises the level without a flag, for CI and shell aliases.
const verbosityEnv = "ACK_VERBOSITY"

// resolveVerbosity prefers an explicit -v over the environment, so a flag can turn
// diagnostics back down.
func resolveVerbosity(cmd *cobra.Command) (int, error) {
	if cmd.Flags().Changed("verbosity") {
		return verbosity, nil
	}
	raw, ok := os.LookupEnv(verbosityEnv)
	if !ok || raw == "" {
		return verbosity, nil
	}
	level, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: want an integer", verbosityEnv, raw)
	}
	return level, nil
}

// listCmd groups the read-only catalog views, which make no AWS calls.
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show what ackctl knows about ACK resources",
	Long: `list reports on the resource catalog embedded in this binary. All of its
subcommands are read-only and make no AWS API calls, so they work offline and
without credentials.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
}

// versionCmd adds `ackctl version`; cobra wires up only the --version flag.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), rootCmd.Version)
		return err
	},
}

// SetVersion records build metadata, injected at link time from main.
func SetVersion(version, commit string) {
	rootCmd.Version = describeVersion(version, commit, moduleVersion())
}

// describeVersion prefers the linker-injected tag, since `go install` applies no ldflags
// and leaves only the module version the toolchain records.
func describeVersion(version, commit, module string) string {
	if version == "dev" && module != "" {
		return module
	}
	return version + " (" + commit + ")"
}

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "(devel)" {
		return ""
	}
	return info.Main.Version
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// verbosity is bound to the persistent -v flag.
var verbosity int

func init() {
	rootCmd.PersistentFlags().IntVarP(&verbosity, "verbosity", "v", debuglog.LevelQuiet,
		"diagnostic detail on stderr: 1 explains an unexpected result, 2 traces every "+
			"request and resolution (also ACK_VERBOSITY)")

	// The catalog views are read while deciding whether to adopt, so they sit under
	// "list" and not under "adopt".
	listCmd.AddCommand(adoptableCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(versionCmd)
}
