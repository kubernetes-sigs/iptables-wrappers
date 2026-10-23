# iptables-wrappers

This repository contains helpers for using iptables in containers. In
particular, it helps Kubernetes pods that want to use iptables in the
host network namespace deal with figuring out whether the host network
namespace is using "legacy" iptables or the modern nft-based version
of iptables.

There are 2 ways to use iptables-wrappers:

  - You can build the `iptables-wrapper` binary and install it into
    your container image, as described below, and then just call
    `iptables` (and `iptables-save`, etc.) just like you normally
    would. The `iptables-wrapper` binary will intercept the first call
    to any iptables binary, figure out the system iptables mode, and
    then update the container's filesystem so that further calls to
    `iptables`, etc, will directly invoke the legacy or nft binaries
    as appropriate.

  - You can vendor this module and then use its
    `"sigs.k8s.io/iptables-wrappers/pkg/iptables"` module to make
    iptables calls, passing `iptables.DetectHostMode` to
    `iptables.New()`/`iptables.NewBestEffort()` to make the
    `iptables.Interface` methods automatically use the legacy or nft
    binaries as appropriate.

  - Alternatively, for binaries that run as system processes (like
    kubelet or CNI binaries) that just want to invoke `/sbin/iptables`
    directly; or for pod-network processes that just want to use
    whatever iptables binary is in their container image, you can use
    construct the `iptables.Interface` with
    `iptables.UseInstalledMode`.

## Background

As of iptables 1.8, the iptables command line clients come in two
different versions/modes: "legacy", which uses the kernel iptables API
just like iptables 1.6 and earlier did, and "nft", which translates
the iptables command-line API into the kernel nftables API.

Because they connect to two different subsystems in the kernel, you
cannot mix and match between them; in particular, if you are adding a
new rule that needs to run either before or after some existing rules
(such as the system firewall rules), then you need to create your rule
with the same iptables mode as the other rules were created with,
since otherwise the ordering may not be what you expect. (eg, if you
*prepend* a rule using the nft-based client, it will still run *after*
all rules that were added with the legacy iptables client.)

In particular, this means that if you create a container image that
will make changes to iptables rules in the host network namespace, and
you want that container to be able to work on any host, then you need
to figure out at run time which mode the host is using, and then also
use that mode yourself. This wrapper is designed to do that for you.

## iptables-wrapper

To build a container image that will run iptables in the host network
namespace:

  - Install iptables 1.8.4 or later in the container, both nft and
    legacy versions, using whatever tools you normally would.

  - Build `iptables-wrapper`, and copy it into the same directory as
    the `iptables-legacy` and `iptables-nft` binaries.

  - Run `iptables-wrapper install` to complete the installation. (On
    distros with an "alternatives" system, this will register the
    `iptables-wrapper` binary as a high-priority alternative to
    `iptables-legacy` and `iptables-nft`. On distros without such a
    system, it will just re-point the `iptables`, `iptables-save`,
    etc, symlinks manually.)

While there are no actual "releases" of iptables-wrappers, you can use
the [`v4`] tag if you want a known-good version of the binary.

[`v4`]: https://github.com/kubernetes-sigs/iptables-wrappers/tree/v3

The first time the wrapper is run, it will figure out which mode the
system is using, update the `iptables`, `iptables-save`, etc, links to
point to either the nft or legacy copies of iptables as appropriate,
and then exec the appropriate binary. After that first call, the
wrapper will not be used again; future calls to iptables will go
directly to the correct underlying binary.

`iptables-wrapper` has no dependencies on anything other than the
iptables binaries themselves, to help you keep your image as small as
possible. The kubernetes [`distroless-iptables`] image is one example
of installing Debian iptables binaries on a "distroless" base image.

[`distroless-iptables`]: https://github.com/kubernetes/release/tree/v0.22.0/images/build/distroless-iptables

Some distro-specific installation examples:

- Alpine Linux

      FROM alpine:3.23

      RUN apk add --no-cache iptables iptables-legacy

      COPY bin/iptables-wrapper /usr/sbin
      RUN /usr/sbin/iptables-wrapper install

- Debian GNU/Linux

      FROM debian:trixie

      RUN apt-get -y --no-install-recommends install iptables

      COPY bin/iptables-wrapper /usr/sbin
      RUN /usr/sbin/iptables-wrapper install

- Fedora

      FROM fedora:43

      RUN dnf install -y iptables iptables-legacy iptables-nft

      COPY bin/iptables-wrapper /usr/bin
      RUN /usr/bin/iptables-wrapper install

- RHEL / CentOS / UBI

  RHEL 7 ships iptables 1.4, which does not support nft mode. RHEL 8
  and later ship a hacked version of iptables 1.8 that *only*
  supports nft mode. Therefore, neither can be used as a basis for a
  portable iptables-using container image.

## `pkg/iptables` (and `pkg/xtables`)

The module `"sigs.k8s.io/iptables-wrappers/pkg/iptables"` is a copy of
Kubernetes's `pkg/util/iptables`, with some minor API updates (in
particular, to reduce its dependency footprint). Additionally, as
noted above, the constructors take a `mode` argument that can be used
to specify that it should or should not do mode auto-detection.

Note that `pkg/iptables` is considered mostly "done", and we have no
plans to extend its API further.

`"sigs.k8s.io/iptables-wrappers/pkg/xtables"` contains the actual
`DetectMode` function that does legacy/nft detection, as well as an
object, `xtables.ExecWrapper` that implements `exec.Interface` from
`"k8s.io/utils/exec"`, but automatically rewrites all invocations of
iptables-related binaries to use a particular mode. This is used
internally by `pkg/iptables`, but could also be used independently.

