/*
Copyright 2022 The Kubernetes Authors.

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
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var (
	regexChain   = regexp.MustCompile(`(?s):(KUBE-IPTABLES-HINT|KUBE-KUBELET-CANARY)`)
	regexIPTVers = regexp.MustCompile(`(?s)v1.8.[0123]`)
)

const (
	// As we are going directly to distroless, we can assume the binary will be at
	// `/usr/sbin`
	iptablesNFTPath           = "/usr/sbin/xtables-nft-multi"
	iptablesLegacyPath        = "/usr/sbin/xtables-legacy-multi"
	iptablesNFTMode    string = "nft"
	iptablesLegacyMode string = "legacy"
)

// TODO: Move this comment to a better place
/*
Assuming that when assembling the container the following binaries will be linked to
this wrapper:
* iptables
* iptables-wrapper
* iptables-save
* iptables-restore
* ip6tables
* ip6tables-save
* ip6tables-restore

1) This binary will try to detect which iptables mode was being used by kubelet,
calling `xtables-<mode>-multi` and checking if the kubelet rules exists in this path
2) Determined the right mode/binary, it will call `xtables-<mode>-multi` passing
the command and the arguments to it.
*/
func main() {
	if err := checkIptablesNFTVersion(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	iptablesMode, err := getIptablesMode()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	var iptablesBinary string

	switch iptablesMode {
	case iptablesNFTMode:
		iptablesBinary = iptablesNFTPath
	case iptablesLegacyMode:
		iptablesBinary = iptablesLegacyPath
	default:
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	args := make([]string, 0)
	args = append(args, filepath.Base(os.Args[0]))
	if len(os.Args) > 0 {
		args = append(args, os.Args...)
	}

	// Buffers are faster than standard string reading
	var output bytes.Buffer

	cmdIptables := &exec.Cmd{
		Path:   iptablesBinary,
		Args:   args,
		Stdout: &output,
	}

	if err := cmdIptables.Run(); err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Print(output.String())

}

func checkIptablesNFTVersion() error {
	outNft, err := exec.Command(iptablesNFTPath, "iptables", "--version").Output()
	if err != nil {
		return err
	}
	if regexIPTVers.Match(outNft) {
		return errors.New("ERROR: iptables 1.8.0 - 1.8.3 have compatibility bugs. Upgrade to v1.8.4 or newer.")
	}
	return nil
}

// getIptablesMode uses the tries to find the pre-existing "KUBE-IPTABLES-HINT"
// or "KUBE-KUBELET-CANARY" to determine which iptables mode should be used
func getIptablesMode() (string, error) {
	/*
		In kubernetes 1.17 and later, kubelet will have created at least
		one chain in the "mangle" table (either "KUBE-IPTABLES-HINT" or
		"KUBE-KUBELET-CANARY"), so check that first, against
		iptables-nft, because we can check that more efficiently and
		it's more common these days.
	*/

	outNft, err := exec.Command(iptablesNFTPath, "iptables-save",
		"-t", "mangle").Output()

	if err != nil {
		return "", err
	}

	outNftv6, err := exec.Command(iptablesNFTPath, "ip6tables-nft-save",
		"-t", "mangle").Output()

	if err != nil {
		return "", err
	}

	if hasKubernetesChains(outNft) || hasKubernetesChains(outNftv6) {
		return iptablesNFTMode, nil
	}

	// Per @danwinship comment, we can deprecate the logic of getting NFT vs Legacy
	// rules and assume if it's not NFT, then it's legacy and the rules should have
	// been created in Legacy
	outLegacy, err := exec.Command(iptablesLegacyPath, "iptables-legacy-save").Output()
	if err != nil {
		return "", err
	}

	outLegacyv6, err := exec.Command(iptablesLegacyPath, "ip6tables-legacy-save").Output()
	if err != nil {
		return "", err
	}

	// If we find any rules in NFT, then NFT mode will be used
	if hasKubernetesChains(outLegacy) || hasKubernetesChains(outLegacyv6) {
		return iptablesLegacyMode, nil
	}

	return "", errors.New("no rule could be detected")

}

// hasKubernetesChains tries to find in the output of the binary if the Kubernetes
// chains exists
func hasKubernetesChains(output []byte) bool {
	return regexChain.Match(output)
}
