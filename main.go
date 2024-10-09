/*
Copyright 2023 The Kubernetes Authors.
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

/*
Iptables-wrapper tries to detect which iptables mode is being used by the host
even when being run from a container. It then updates the iptables commands to
point to the right binaries for that mode. Before exiting it re-executes the given
command.

The process is as follows:
 1. Calls `xtables-<mode>-multi` and checks if the kubelet rules exists.
    It searches for different patterns in the configured rules, trying to match different
    kubernetes versions, and it uses the results to guess which mode is in use.
 2. Updates the alternatives/symlinks to point to the proper binaries for the detected mode.
    Depending on the OS it uses `update-alternatives`, `alternatives` or it manually creates symlinks.
 3. Re-execs the original command received by this binary.

We assume this binary has been symlinked to some/all iptables binaries and whatever was received
here was intended to be an iptables-* command. If that is not the case and this command is either
executed directly or through a symlink that doesn't point to an iptables binary,
it will enter an infinite loop, calling itself recursively.

It's important to note that this proxy behavior will only happen on the first iptables-*
execution. Following invocations will use directly the binaries for the selected mode.

If the command is executed with the `install` argument, it will create symlinks for all iptables binaries
pointing to itself. The second argument must be the path where the symlinks should be installed, preferably
the same path where iptables is installed.
This is useful for the installation process of the iptables-wrapper itself before first execution.
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubernetes-sigs/iptables-wrappers/internal/iptables"
)

func main() {
	ctx := context.Background()

	if installMode() {
		install(ctx)
		return
	}

	forward(ctx)
}

func installMode() bool {
	return len(os.Args) == 3 && os.Args[1] == "install"
}

func installFolder() string {
	return os.Args[2]
}

// install creates symlinks for all iptables binaries in the given folder
// pointing to the current binary being executed.
func install(ctx context.Context) {
	wrapperPath, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	wrapperPath = filepath.Clean(wrapperPath)

	if err := iptables.NewSymlinker(installFolder()).LinkAll(ctx, wrapperPath); err != nil {
		fatal(err)
	}
}

// forward detects the iptables mode to use and re-executes the exact same command passed to this program.
func forward(ctx context.Context) {
	sbinPath, err := iptables.DetectBinaryDir()
	if err != nil {
		fatal(err)
	}

	// We use `xtables-<mode>-multi` binaries by default to inspect the installed rules,
	// but this can be changed to directly use `iptables-<mode>-save` binaries.
	mode := iptables.DetectMode(ctx, iptables.NewXtablesMultiInstallation(sbinPath))

	// This re-executes the exact same command passed to this program
	binaryPath := os.Args[0]
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	selector := iptables.BuildAlternativeSelector(sbinPath)
	if err := selector.UseMode(ctx, mode); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to redirect iptables binaries. (Are you running in an unprivileged pod?): %s\n", err)
		// fake it, though this will probably also fail if they aren't root
		binaryPath = iptables.XtablesPath(sbinPath, mode)
		args = os.Args
	}

	cmdIPTables := exec.CommandContext(ctx, binaryPath, args...)
	cmdIPTables.Stdout = os.Stdout
	cmdIPTables.Stderr = os.Stderr

	if err := cmdIPTables.Run(); err != nil {
		code := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			// If it's not an ExitError, the command probably didn't finish and something
			// else failed, which means it might not had outputted anything. In that case,
			// print the error message just in case.
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		}
		os.Exit(code)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
