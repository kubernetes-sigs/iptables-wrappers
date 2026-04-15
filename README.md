# iptables-wrappers

This repository consists of wrappers to help with using
iptables in containers.

Specifically, it provides a wrapper to select between the two
modes of iptables 1.8 ("legacy" and "nft") at runtime, so that
hostNetwork containers that examine or modify iptables rules will work
correctly regardless of which mode the underlying system is using.

This wrapper is only compatible with Kubernetes 1.17 and newer versions.
If you need to support older releases, then use the original shell-script
version of iptables-wrappers, from the `[v2]` tag in this repository.

[v2]: https://github.com/kubernetes-sigs/iptables-wrappers/tree/v2

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

The `iptables-wrapper-installer.sh` script in this repo will install
an `iptables-wrapper` binary alongside `iptables-legacy` and
`iptables-nft` in `/usr/sbin` (or `/sbin`), and adjust the symlinks on
`iptables`, `iptables-save`, etc, to point to the wrapper.

(Because of the known bugs, `iptables-wrapper-installer.sh` will
refuse to install the wrappers into a container with iptables earlier
than 1.8.4. If you really know what you're doing you can pass
`--no-sanity-check` to install anyway.)

The first time the wrapper is run, it will figure out which mode the
system is using, update the `iptables`, `iptables-save`, etc, links to
point to either the nft or legacy copies of iptables as appropriate,
and then exec the appropriate binary. After that first call, the
wrapper will not be used again; future calls to iptables will go
directly to the correct underlying binary.

## Building a container image that uses iptables

When building a container image that needs to run iptables in the host
network namespace, install iptables 1.8.4 or later in the container
using whatever tools you normally would. Then copy the
[`iptables-wrapper-installer.sh`](./iptables-wrapper-installer.sh)
script alongside the compiled `iptables-wrapper` binary into your
container, and run it to have it set up run-time autodetection.

Some distro-specific examples:

- Alpine Linux

      FROM alpine:3.23

      RUN apk add --no-cache iptables iptables-legacy
      COPY iptables-wrapper-installer.sh /
      COPY bin/iptables-wrapper /
      RUN /iptables-wrapper-installer.sh

- Debian GNU/Linux

      FROM debian:trixie

      RUN apt-get -y --no-install-recommends install iptables

      COPY iptables-wrapper-installer.sh /
      COPY bin/iptables-wrapper /
      RUN /iptables-wrapper-installer.sh

- Fedora

      FROM fedora:43

      RUN dnf install -y iptables iptables-legacy iptables-nft

      COPY iptables-wrapper-installer.sh /
      COPY bin/iptables-wrapper /
      RUN /iptables-wrapper-installer.sh

- RHEL / CentOS / UBI

  RHEL 7 ships iptables 1.4, which does not support nft mode. RHEL 8
  and later ship a hacked version of iptables 1.8 that *only*
  supports nft mode. Therefore, neither can be used as a basis for a
  portable iptables-using container image.
