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
	"path/filepath"
)

// Mode represents the two different modes iptables can be configured in: nft or legacy.
// The string value is identical to the form used in iptables command names (e.g.
// "iptables-legacy", "ip6tables-nft-save", "xtables-nft-multi")
type Mode string

const (
	LegacyMode Mode = "legacy"
	NFTMode    Mode = "nft"
)

// IPTablesBinaries is the list of aliases for the xtables-*-multi binaries
var IPTablesBinaries = []string{
	"iptables",
	"iptables-save",
	"iptables-restore",
	"ip6tables",
	"ip6tables-save",
	"ip6tables-restore",
}

// XtablesPath returns the path to the `xtables-<mode>-multi` binary
func XtablesPath(sbinPath string, mode Mode) string {
	return filepath.Join(sbinPath, "xtables-"+string(mode)+"-multi")
}
