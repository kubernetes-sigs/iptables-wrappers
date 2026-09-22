# iptables-wrappers

This repository consists of wrappers to help with using
iptables in containers.

Specifically, it provides a wrapper to select between the two
modes of iptables 1.8 ("legacy" and "nft") at runtime, so that
hostNetwork containers that examine or modify iptables rules will work
correctly regardless of which mode the underlying system is using.

While there are no actual "releases" of iptables-wrappers, there have
been three tagged versions (plus an intermediate untagged version that
was used by some consumers):

  - [v1], 2020-01-12: The original shell-script-based version, which
    compared the line count of `iptables-legacy-save` and
    `iptables-nft-save` to guess which version was in use, with a
    workaround for a bug in iptables 1.8.3. Defaulted to assuming
    "legacy" if it couldn't determine which was in use.

  - [v2], 2022-04-12: Required iptables 1.8.4 or later, and improved
    legacy-vs-nft detection in Kubernetes 1.17 and later by looking
    for specific iptables chains created by kubelet. Also switched
    the default/fallback guess from "legacy" to "nft".

  - (untagged), 2023-04-01: Rewritten in go, to better support
    distroless containers. Dropped support for Kubernetes < 1.17. (Not
    initially tagged as `v3` while we figured out how to update the
    installer and unit tests, which then got forgotten about.)

  - [v3], 2026-04-20: Official release of the go version.

[v1]: https://github.com/kubernetes-sigs/iptables-wrappers/tree/v1
[v2]: https://github.com/kubernetes-sigs/iptables-wrappers/tree/v2
[v3]: https://github.com/kubernetes-sigs/iptables-wrappers/tree/v3

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

The first time the wrapper is run, it will figure out which mode the
system is using, update the `iptables`, `iptables-save`, etc, links to
point to either the nft or legacy copies of iptables as appropriate,
and then exec the appropriate binary. After that first call, the
wrapper will not be used again; future calls to iptables will go
directly to the correct underlying binary.

`iptables-wrapper` has no dependencies on anything other than the
iptables binaries themselves, to help you keep your image as small as
possible.
https://github.com/kubernetes/release/blob/master/images/build/distroless-iptables/
is one example of installing Debian iptables binaries on a
"distroless" base image.

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
