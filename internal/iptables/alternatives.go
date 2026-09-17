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

package iptables

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sigs.k8s.io/iptables-wrappers/internal/commands"
	"sigs.k8s.io/iptables-wrappers/internal/files"
)

// SetIPTablesAlternative updates the system to use the given iptables mode
func SetIPTablesAlternative(ctx context.Context, mode Mode, sbinPath string) error {
	modeStr := string(mode)

	if files.ExecutableExists(filepath.Join(sbinPath, "alternatives")) {
		// Fedora-style "alternatives".
		if err := commands.RunAndReadError(exec.CommandContext(ctx, "alternatives", "--set", "iptables", filepath.Join(sbinPath, "iptables-"+string(mode)))); err != nil {
			return fmt.Errorf("alternatives to update iptables to mode %s: %v", string(mode), err)
		}
		return nil
	} else if files.ExecutableExists(filepath.Join(sbinPath, "update-alternatives")) {
		// Debian-style "update-alternatives".
		if err := commands.RunAndReadError(exec.CommandContext(ctx, "update-alternatives", "--set", "iptables", filepath.Join(sbinPath, "iptables-"+modeStr))); err != nil {
			return fmt.Errorf("update-alternatives iptables to mode %s: %v", modeStr, err)
		}
		if err := commands.RunAndReadError(exec.CommandContext(ctx, "update-alternatives", "--set", "ip6tables", filepath.Join(sbinPath, "ip6tables-"+modeStr))); err != nil {
			return fmt.Errorf("update-alternatives ip6tables to mode %s: %v", modeStr, err)
		}
		return nil
	} else {
		// If we don't find any tool to manage alternatives, handle it manually with symlinks.
		return LinkAll(ctx, sbinPath, IPTablesBinaries, XtablesPath(sbinPath, mode))
	}
}

// LinkAll creates symlinks from each element of linknames in binaryPath, to targetBinary.
func LinkAll(ctx context.Context, binaryPath string, linknames []string, targetBinary string) error {
	for _, cmd := range linknames {
		cmdPath := filepath.Join(binaryPath, cmd)
		// If deleting fails, ignore it and try to create symlink regardless
		_ = os.RemoveAll(cmdPath)

		if err := os.Symlink(targetBinary, cmdPath); err != nil {
			return fmt.Errorf("creating %s symlink to %s: %v", cmd, targetBinary, err)
		}
	}

	return nil
}
