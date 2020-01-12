all:

check: check-debian check-debian-nosanity check-debian-backports check-fedora check-alpine

check-debian:
	./test/run-test.sh --build-fail debian

check-debian-nosanity:
	./test/run-test.sh --build-arg="INSTALL_ARGS=--no-sanity-check" --nft-fail debian-nosanity

check-debian-backports:
	./test/run-test.sh --build-arg="REPO=buster-backports" debian-backports

check-fedora:
	./test/run-test.sh fedora

check-alpine:
	./test/run-test.sh alpine
