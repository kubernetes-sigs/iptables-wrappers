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
	"strings"
	"testing"

	utilexec "k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

type mewTestCommand struct {
	original    string
	translation string
}

func TestExecWrapper(t *testing.T) {
	testCases := []struct {
		name          string
		mode          Mode
		testCommands  []mewTestCommand
	}{
		{
			name: "legacy mode",
			mode: LegacyMode,
			testCommands: []mewTestCommand{
				{
					original:    "iptables -N FOO",
					translation: "/sbin/iptables-legacy -N FOO",
				},
				{
					original:    "/sbin/iptables -N FOO",
					translation: "/sbin/iptables-legacy -N FOO",
				},
				{
					original:    "/usr/sbin/iptables -N FOO",
					translation: "/sbin/iptables-legacy -N FOO",
				},
				{
					original:    "ip6tables-restore",
					translation: "/sbin/ip6tables-legacy-restore",
				},
				{
					original:    "iptables-translate -A FOO -j DROP",
					translation: "iptables-translate -A FOO -j DROP",
				},
				{
					original:    "iptables-nft-save",
					translation: "iptables-nft-save",
				},
				{
					original:    "iptables-legacy-save",
					translation: "iptables-legacy-save",
				},
			},
		},
		{
			name: "nft mode",
			mode: NFTMode,
			testCommands: []mewTestCommand{
				{
					original:    "iptables -N FOO",
					translation: "/sbin/iptables-nft -N FOO",
				},
				{
					original:    "/sbin/iptables -N FOO",
					translation: "/sbin/iptables-nft -N FOO",
				},
				{
					original:    "/usr/sbin/iptables -N FOO",
					translation: "/sbin/iptables-nft -N FOO",
				},
				{
					original:    "ip6tables-restore",
					translation: "/sbin/ip6tables-nft-restore",
				},
				{
					original:    "iptables-translate -A FOO -j DROP",
					translation: "iptables-translate -A FOO -j DROP",
				},
				{
					original:    "iptables-nft-save",
					translation: "iptables-nft-save",
				},
				{
					original:    "iptables-legacy-save",
					translation: "iptables-legacy-save",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			unDetectMode()
			fexec := fakeExecForCommands(nil, "")
			xt := NewExecWrapper(fexec, tc.mode)

			for _, command := range tc.testCommands {
				fcmd := fakeexec.FakeCmd{
					RunScript: []fakeexec.FakeAction{func() ([]byte, []byte, error) { return nil, nil, nil }},
				}
				argv := strings.Fields(command.translation)
				fexec.CommandScript = append(fexec.CommandScript,
					func(cmd string, args ...string) utilexec.Cmd {
						return fakeexec.InitFakeCmd(&fcmd, argv[0], argv[1:]...)
					},
				)

				origArgv := strings.Fields(command.original)
				cmd := xt.CommandContext(t.Context(), origArgv[0], origArgv[1:]...)
				err := cmd.Run()
				if err != nil {
					t.Errorf("Unexpected error running %q: %v", command, err)
				}
				// We don't need to do any other checking; the FakeExec will
				// fail if the command translation was wrong.
			}
		})
	}
}
