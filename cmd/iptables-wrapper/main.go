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
iptables-wrapper tries to detect which iptables mode is being used by the host when being
run from a container. It then updates the iptables commands to point to the right binaries
for that mode and then executes the original command with the correct binary.

The process is as follows:

 1. Calls `xtables-<mode>-multi` and checks if the kubelet rules exists.

 2. Updates the alternatives/symlinks to point to the proper binaries for the detected
    mode. Depending on the OS, it uses `update-alternatives` or `alternatives`, or it
    manually creates symlinks.

 3. Re-execs the original command received by this binary.

Normally, iptables-wrapper will only run once, and further invocations of iptables will go
directly to the appropriate legacy or nft iptables binaries. However, if step 2 fails
(generally because the filesystem is read-only), then iptables-wrappers will still
manually exec the correct legacy/nft binary, and then later invocations of iptables would
still go to iptables-wrappers, and it would have to re-detect the underlying mode each
time.

If the command is executed with the `install` argument, then instead of its normal
behavior, it will create symlinks for all iptables binaries pointing to itself. It must be
invoked by its full path, and it will use that path for the iptables binaries as well.
This is useful for the installation process of the iptables-wrapper itself before first
execution.
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	utilexec "k8s.io/utils/exec"

	"sigs.k8s.io/iptables-wrappers/pkg/xtables"
)

func main() {
	ctx := context.Background()
	execer := utilexec.New()

	sbinPath, err := xtables.DetectBinaryDir(execer)
	if err != nil {
		fatal(err)
	}
	alternatives := newAlternativesSelector(sbinPath)

	if len(os.Args) == 2 && os.Args[1] == "install" {
		install(ctx, alternatives)
		return
	}

	mode := xtables.DetectMode(ctx, execer, sbinPath)

	// Extract the original command and arguments.
	binaryPath := os.Args[0]
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	if err := alternatives.Update(ctx, mode); err != nil {
		// Following the example of iptables's own "legacy tables present"
		// warning, print the error message as a "comment", so it won't interfere
		// with iptables-save parsing.
		fmt.Fprintf(os.Stderr, "# iptables-wrapper: unable to redirect iptables binaries:\n")
		for _, line := range strings.Split(strings.TrimSpace(err.Error()), "\n") {
			fmt.Fprintf(os.Stderr, "#   %s\n", line)
		}

		// As a fallback, pass the command to the appropriate multi-binary
		binaryPath = xtables.MultiBinaryPath(sbinPath, mode)
		args = os.Args
		args[0] = filepath.Base(args[0])
	}

	cmdIPTables := execer.CommandContext(ctx, binaryPath, args...)
	cmdIPTables.SetStdin(os.Stdin)
	cmdIPTables.SetStdout(os.Stdout)
	cmdIPTables.SetStderr(os.Stderr)

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

// install creates symlinks for all iptables binaries in the same directory as the
// existing iptables binaries.
func install(ctx context.Context, alternatives alternativesSelector) {
	wrapperPath, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	wrapperPath = filepath.Clean(wrapperPath)
	if err := alternatives.Install(ctx, wrapperPath); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
