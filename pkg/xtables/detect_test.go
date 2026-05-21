/*
Copyright The Kubernetes Authors.

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
	"fmt"
	"io"
	"strings"
	"testing"

	utilexec "k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

const rulesWithIPTablesHint = `# Blah blah blah
*filter
:INPUT ACCEPT [0:0]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:DOCKER - [0:0]
:DOCKER-BRIDGE - [0:0]
:DOCKER-FORWARD - [0:0]
:KUBE-IPTABLES-HINT - [0:0]
-A FORWARD -j DOCKER-FORWARD
-A DOCKER ! -i br-309f9ae3f545 -o br-309f9ae3f545 -j DROP
-A DOCKER ! -i docker0 -o docker0 -j DROP
-A DOCKER-BRIDGE -o br-309f9ae3f545 -j DOCKER
-A DOCKER-BRIDGE -o docker0 -j DOCKER
-A DOCKER-FORWARD -j DOCKER-BRIDGE
-A DOCKER-FORWARD -i br-309f9ae3f545 -j ACCEPT
-A DOCKER-FORWARD -i br-362465bc55a2 -o br-362465bc55a2 -j ACCEPT
-A DOCKER-FORWARD -i docker0 -j ACCEPT
COMMIT
`

const rulesWithoutIPTablesHint = `# Blah blah blah
*filter
:INPUT ACCEPT [0:0]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:DOCKER - [0:0]
:DOCKER-BRIDGE - [0:0]
:DOCKER-FORWARD - [0:0]
-A FORWARD -j DOCKER-FORWARD
-A DOCKER ! -i br-309f9ae3f545 -o br-309f9ae3f545 -j DROP
-A DOCKER ! -i docker0 -o docker0 -j DROP
-A DOCKER-BRIDGE -o br-309f9ae3f545 -j DOCKER
-A DOCKER-BRIDGE -o docker0 -j DOCKER
-A DOCKER-FORWARD -j DOCKER-BRIDGE
-A DOCKER-FORWARD -i br-309f9ae3f545 -j ACCEPT
-A DOCKER-FORWARD -i br-362465bc55a2 -o br-362465bc55a2 -j ACCEPT
-A DOCKER-FORWARD -i docker0 -j ACCEPT
COMMIT
`

type testCommand struct {
	command string
	stdout  string
	err     error
}

// Creates a FakeExec that expects exactly commands to be run (and will fail otherwise).
func fakeExecForCommands(commands []testCommand) *fakeexec.FakeExec {
	fexec := &fakeexec.FakeExec{
		CommandScript: make([]fakeexec.FakeCommandAction, len(commands)),
		ExactOrder:    true,
	}
	for i := range commands {
		fcmd := fakeexec.FakeCmd{
			RunScript: []fakeexec.FakeAction{func() ([]byte, []byte, error) { return nil, nil, commands[i].err }},
			StdoutPipeResponse: fakeexec.FakeStdIOPipeResponse{
				ReadCloser: io.NopCloser(strings.NewReader(commands[i].stdout)),
			},
		}
		argv := strings.Fields(commands[i].command)
		fexec.CommandScript[i] = func(cmd string, args ...string) utilexec.Cmd {
			return fakeexec.InitFakeCmd(&fcmd, argv[0], argv[1:]...)
		}
	}
	return fexec
}

func TestDetectMode(t *testing.T) {
	testCases := []struct {
		name     string
		commands []testCommand
		mode     Mode
	}{
		{
			name: "iptables broken",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					err:     fmt.Errorf("oh noes!"),
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					err:     fmt.Errorf("oh noes!"),
				},
				{
					command: "/sbin/iptables-legacy-save",
					err:     fmt.Errorf("oh noes!"),
				},
				{
					command: "/sbin/ip6tables-legacy-save",
					err:     fmt.Errorf("oh noes!"),
				},
			},
			mode: NFTMode,
		},
		{
			name: "no rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-legacy-save",
					stdout:  "",
				},
			},
			mode: NFTMode,
		},
		{
			name: "old legacy rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  ":KUBE-KUBELET-CANARY - [0:0]\n",
				},
			},
			mode: LegacyMode,
		},
		{
			name: "new legacy rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: LegacyMode,
		},
		{
			name: "old nft rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  ":KUBE-KUBELET-CANARY - [0:0]\n",
				},
			},
			mode: NFTMode,
		},
		{
			name: "new nft rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: NFTMode,
		},
		{
			name: "legacy, no ipv6",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					err:     fmt.Errorf("blah blah no ipv6"),
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: LegacyMode,
		},
		{
			name: "legacy, ipv6-only",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-legacy-save",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: LegacyMode,
		},
		{
			name: "nft, ipv6-only",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: NFTMode,
		},
		{
			name: "nft, lots of rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  rulesWithIPTablesHint,
				},
			},
			mode: NFTMode,
		},
		{
			name: "non-Kubernetes nft rules, Kubernetes legacy rules",
			commands: []testCommand{
				{
					command: "/sbin/iptables-nft-save -t mangle",
					stdout:  rulesWithoutIPTablesHint,
				},
				{
					command: "/sbin/ip6tables-nft-save -t mangle",
					stdout:  "",
				},
				{
					command: "/sbin/iptables-legacy-save",
					stdout:  ":KUBE-IPTABLES-HINT - [0:0]\n",
				},
			},
			mode: LegacyMode,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fexec := fakeExecForCommands(tc.commands)

			mode := DetectMode(t.Context(), fexec, "/sbin")
			if mode != tc.mode {
				t.Errorf("Expected mode %q got %q", tc.mode, mode)
			}
		})
	}
}

func fakeExecForReader(reader io.Reader) *fakeexec.FakeExec {
	fexec := &fakeexec.FakeExec{CommandScript: make([]fakeexec.FakeCommandAction, 4)}
	// DetectMode runs up to 4 iptables commands; we use reader only for the first
	for i := range 4 {
		var rc io.ReadCloser
		if i == 0 {
			rc = io.NopCloser(reader)
		} else {
			rc = io.NopCloser(&bytes.Buffer{})
		}
		fexec.CommandScript[i] = func(cmd string, args ...string) utilexec.Cmd {
			return &fakeexec.FakeCmd{
				RunScript: []fakeexec.FakeAction{func() ([]byte, []byte, error) { return nil, nil, nil }},
				StdoutPipeResponse: fakeexec.FakeStdIOPipeResponse{
					ReadCloser: rc,
				},
			}
		}
	}
	return fexec
}

func Test_execAndScanForKubeletChains(t *testing.T) {
	matching := &bytes.Buffer{}
	nonMatching := &bytes.Buffer{}
	for matching.Len() < 16384 {
		_, _ = matching.WriteString(rulesWithoutIPTablesHint)
		_, _ = nonMatching.WriteString(rulesWithoutIPTablesHint)
	}
	// Write the block with the hint only to matching
	_, _ = matching.WriteString(rulesWithIPTablesHint)
	for matching.Len() < 65536 {
		_, _ = matching.WriteString(rulesWithoutIPTablesHint)
		_, _ = nonMatching.WriteString(rulesWithoutIPTablesHint)
	}

	testCases := []struct {
		name      string
		buf       *bytes.Buffer
		expectEOF bool
	}{
		{
			name:      "DetectMode stops before EOF on a match",
			buf:       matching,
			expectEOF: false,
		},
		{
			name:      "DetectMode reads to EOF when no match",
			buf:       nonMatching,
			expectEOF: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fexec := fakeExecForReader(tc.buf)
			mode := DetectMode(t.Context(), fexec, "/sbin")
			if mode != NFTMode {
				t.Errorf("Expected mode %q got %q", NFTMode, mode)
			}

			reachedEOF := tc.buf.Len() == 0
			if tc.expectEOF && !reachedEOF {
				t.Errorf("failed to read all input with non-matching data")
			} else if !tc.expectEOF && reachedEOF {
				t.Errorf("read all of the input despite matching data")
			}
		})
	}
}
