# ChangeLog

The `iptables-wrappers` module has two separate streams of release
tags. Releases of the `iptables-wrapper` binary/script are tagged
`iptables-wrapper-v1`, `iptables-wrapper-v2`, etc., while releases of
the vendorable `pkg/iptables`/`pkg/xtables` modules have semantic
versions (e.g. `v1.0.0`), so as to work correctly with `go mod`. Note
that the two sets of version numbers are not comparable. (E.g.,
`pkg/iptables v1.0.0` is newer than `iptables-wrapper-v3`.)

For historical compatibility, `iptables-wrapper-v1`,
`iptables-wrapper-v2`, `iptables-wrapper-v3`, and
`iptables-wrapper-v4` are also aliased to `v1`, `v2`, `v3`, and `v4`.
Future binary releases will only have the longer tag names.

## `pkg/iptables v1.0.0` - 2026-10-28

- First release of the `pkg/xtables` and `pkg/iptables` modules (the
  latter being an adapted version of Kubernetes's
  `pkg/util/iptables`), to allow Go programs to detect the iptables
  mode themselves without needing an external `iptables-wrapper`.
  ([#22](https://github.com/kubernetes-sigs/iptables-wrappers/pull/22),
  [#24](https://github.com/kubernetes-sigs/iptables-wrappers/pull/24),
  `@danwinship`)

## `iptables-wrapper-v4` - 2026-10-23

- Fixed multiple bugs that prevented `iptables-wrapper` from working
  correctly in read-only containers. (You shouldn't do this unless
  you're only making a handful of iptables calls though; it's very
  inefficient to have to detect the mode every time you run iptables.)
  ([#19](https://github.com/kubernetes-sigs/iptables-wrappers/pull/19),
  `@toabctl` and
  [#20](https://github.com/kubernetes-sigs/iptables-wrappers/pull/20),
  `@danwinship`)

- Made `iptables-wrapper install` support Debian and Fedora images as
  well.
  ([#21](https://github.com/kubernetes-sigs/iptables-wrappers/pull/21),
  `@danwinship`)

- Fixed a bug that made `iptables-wrapper` create symlinks by hand
  after detecting the iptables mode on Debian, rather than using the
  `alternatives` system to update the symlinks. (part of
  [#20](https://github.com/kubernetes-sigs/iptables-wrappers/pull/20),
  `@danwinship`)

- Made the mode detection stream the `iptables-save` results rather
  than reading them into RAM, to make it more efficient when running
  on a node that already has many iptables rules (e.g., when
  kube-proxy restarts).
  ([#18](https://github.com/kubernetes-sigs/iptables-wrappers/pull/18),
  `@danwinship`)

## `iptables-wrapper-v3` - 2026-04-20

- Ported `iptables-wrapper` from bash to go.
  ([#6](https://github.com/kubernetes-sigs/iptables-wrappers/pull/6),
  `@g-gaston`)

- Removed `iptables-wrapper-installer.sh` and added instructions to
  README.md for installing by hand, including using
  `/sbin/iptables-wrapper install` on distroless systems.
  ([#14](https://github.com/kubernetes-sigs/iptables-wrappers/pull/14),
  `@danwinship`, `@g-gaston`)

## `iptables-wrapper-v2` - 2022-04-12

- Switched default to `nft` when it can't detect the mode.

- Optimized rule detection for Kubernetes 1.17+.

- Dropped workarounds for iptables 1.8.3.

## `iptables-wrapper-v1` - 2020-01-12

Original iptables-wrapper script
