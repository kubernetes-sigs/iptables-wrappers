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

package xtables

import (
	"bufio"
	"context"
	"errors"
	"path/filepath"
	"strings"

	utilexec "k8s.io/utils/exec"
)

// DetectBinaryDir tries to detect the `iptables` location in
// either /usr/sbin or /sbin. If it's not there, it returns an error.
func DetectBinaryDir(execer utilexec.Interface) (string, error) {
	if path, _ := execer.LookPath("/usr/sbin/iptables"); path != "" {
		return "/usr/sbin", nil
	} else if path, _ := execer.LookPath("/sbin/iptables"); path != "" {
		return "/sbin", nil
	} else {
		return "", errors.New("iptables is not present in either /usr/sbin or /sbin")
	}
}

// DetectMode inspects the current iptables entries and tries to
// guess which iptables mode is being used: legacy or nft
func DetectMode(ctx context.Context, execer utilexec.Interface, sbinPath string) Mode {
	// This method ignores all errors, this is on purpose. We execute all commands
	// and try to detect patterns in a best effort basis. If somthing fails,
	// continue with the next step. Worse case scenario if everything fails,
	// default to nft.

	// In kubernetes 1.17 and later, kubelet will have created at least
	// one chain in the "mangle" table (either "KUBE-IPTABLES-HINT" or
	// "KUBE-KUBELET-CANARY"), so check that first, against
	// iptables-nft, because we can check that more efficiently and
	// it's more common these days.
	if execAndScanForKubeletChains(ctx, execer, sbinPath, "iptables-nft-save", "-t", "mangle") {
		return NFTMode
	}
	if execAndScanForKubeletChains(ctx, execer, sbinPath, "ip6tables-nft-save", "-t", "mangle") {
		return NFTMode
	}

	// Check for kubernetes 1.17-or-later with iptables-legacy. We
	// can't pass "-t mangle" to iptables-legacy-save because it would
	// cause the kernel to create that table if it didn't already
	// exist, which we don't want. So we have to grab all the rules.
	if execAndScanForKubeletChains(ctx, execer, sbinPath, "iptables-legacy-save") {
		return LegacyMode
	}
	if execAndScanForKubeletChains(ctx, execer, sbinPath, "ip6tables-legacy-save") {
		return LegacyMode
	}

	// If we can't detect either of the patterns, default to nft.
	return NFTMode
}

// Especially on a restart, there may already be lots of iptables rules, so rather than
// using CombinedOutput() or something simple like that that might end up using a lot of
// memory, we stream the command output, only keeping a line at a time, and stop as soon
// as we find a match.
func execAndScanForKubeletChains(ctx context.Context, execer utilexec.Interface, sbinPath, command string, args ...string) bool {
	cmd := execer.CommandContext(ctx, filepath.Join(sbinPath, command), args...)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return false
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, ":KUBE-IPTABLES-HINT ") ||
			strings.HasPrefix(line, ":KUBE-KUBELET-CANARY ") {
			// SIGPIPE the iptables process
			_ = stdout.Close()
			_ = cmd.Wait()
			return true
		}
	}
	_ = cmd.Wait()
	return false
}
