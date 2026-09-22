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

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sigs.k8s.io/iptables-wrappers/pkg/xtables"
)

// alternativesSelector represents the system's native method for deciding between
// iptables-legacy and iptables-nft.
type alternativesSelector interface {
	// UseMode configures the system to use the selected iptables mode.
	UseMode(ctx context.Context, mode xtables.Mode) error
}

// newAlternativesSelector creates the proper alternativesSelector depending on the
// machine's setup. It will use either `alternatives` or `update-alternatives` if present
// in the sbin folder. If neither is present, it will manage iptables binaries by manually
// creating symlinks.
func newAlternativesSelector(sbinPath string) alternativesSelector {
	// (We must check for "alternatives" first, because Fedora has both binaries but
	// doesn't use the same semantics as Debian, so if we try to use
	// updateAlternativesMethod there, it will give errors about ip6tables.)
	if path, _ := exec.LookPath("alternatives"); path != "" {
		return alternativesMethod{sbinPath: sbinPath}
	} else if path, _ := exec.LookPath("update-alternatives"); path != "" {
		return updateAlternativesMethod{sbinPath: sbinPath}
	} else {
		return symlinkAlternativesMethod{sbinPath: sbinPath}
	}
}

// updateAlternativesMethod manages an iptables setup by using the `update-alternatives` binary.
// This is most common for Debian-based distros.
type updateAlternativesMethod struct {
	sbinPath string
}

func (u updateAlternativesMethod) UseMode(ctx context.Context, mode xtables.Mode) error {
	modeStr := string(mode)

	cmd := exec.CommandContext(ctx, "update-alternatives", "--set", "iptables", filepath.Join(u.sbinPath, "iptables-"+modeStr))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not set iptables to mode %q: %w: %s", modeStr, err, out)
	}

	cmd = exec.CommandContext(ctx, "update-alternatives", "--set", "ip6tables", filepath.Join(u.sbinPath, "ip6tables-"+modeStr))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not set ip6tables to mode %q: %w: %s", modeStr, err, out)
	}

	return nil
}

// alternativesMethod manages an iptables setup by using the `alternatives` binary.
// This is most common for Fedora-based distros.
type alternativesMethod struct {
	sbinPath string
}

func (a alternativesMethod) UseMode(ctx context.Context, mode xtables.Mode) error {
	modeStr := string(mode)

	cmd := exec.CommandContext(ctx, "alternatives", "--set", "iptables", filepath.Join(a.sbinPath, "iptables-"+modeStr))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not set iptables mode to %q: %w: %s", modeStr, err, out)
	}
	return nil
}

func NewSymlinker(sbinPath string) symlinkAlternativesMethod {
	return symlinkAlternativesMethod{sbinPath: sbinPath}
}

// symlinkAlternativesMethod manages an iptables setup by manually creating symlinks for
// all iptables commands.
type symlinkAlternativesMethod struct {
	sbinPath string
}

// UseMode configures the system to use the selected iptables mode.
func (s symlinkAlternativesMethod) UseMode(_ context.Context, mode xtables.Mode) error {
	return s.LinkAll(xtables.MultiBinaryPath(s.sbinPath, mode))
}

// LinkAll creates symlinks for all iptables commands to the targetPath.
func (s symlinkAlternativesMethod) LinkAll(targetPath string) error {
	for _, cmd := range xtables.IPTablesBinaries {
		cmdPath := filepath.Join(s.sbinPath, cmd)
		// If deleting fails, ignore it and try to create symlink regardless
		_ = os.RemoveAll(cmdPath)

		if err := os.Symlink(targetPath, cmdPath); err != nil {
			return fmt.Errorf("creating %s symlink to %s: %v", cmd, targetPath, err)
		}
	}

	return nil
}
