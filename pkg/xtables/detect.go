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
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"regexp"

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
	rulesOutput := &bytes.Buffer{}
	doExec(ctx, execer, rulesOutput, filepath.Join(sbinPath, "iptables-nft-save"), "-t", "mangle")
	if hasKubeletChains(rulesOutput.Bytes()) {
		return NFTMode
	}
	rulesOutput.Reset()
	doExec(ctx, execer, rulesOutput, filepath.Join(sbinPath, "ip6tables-nft-save"), "-t", "mangle")
	if hasKubeletChains(rulesOutput.Bytes()) {
		return NFTMode
	}
	rulesOutput.Reset()

	// Check for kubernetes 1.17-or-later with iptables-legacy. We
	// can't pass "-t mangle" to iptables-legacy-save because it would
	// cause the kernel to create that table if it didn't already
	// exist, which we don't want. So we have to grab all the rules.
	doExec(ctx, execer, rulesOutput, filepath.Join(sbinPath, "iptables-legacy-save"))
	if hasKubeletChains(rulesOutput.Bytes()) {
		return LegacyMode
	}
	rulesOutput.Reset()
	doExec(ctx, execer, rulesOutput, filepath.Join(sbinPath, "ip6tables-legacy-save"))
	if hasKubeletChains(rulesOutput.Bytes()) {
		return LegacyMode
	}

	// If we can't detect either of the patterns, default to nft.
	return NFTMode
}

func doExec(ctx context.Context, execer utilexec.Interface, out *bytes.Buffer, command string, args ...string) {
	c := execer.CommandContext(ctx, command, args...)
	c.SetStdout(out)
	_ = c.Run()
}

var kubeletChainsRegex = regexp.MustCompile(`(?m)^:(KUBE-IPTABLES-HINT|KUBE-KUBELET-CANARY)`)

// hasKubeletChains checks if the output of an iptables*-save command
// contains any of the rules set by kubelet.
func hasKubeletChains(output []byte) bool {
	return kubeletChainsRegex.Match(output)
}
