#!/bin/sh

test_description="ipfs-update install with many different versions"

. lib/test-lib.sh

GUEST_IPFS_UPDATE="sharness/bin/ipfs-update"

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_install_version "v0.3.7"
test_install_version "v0.3.10"
test_install_version "v0.4.0-dev"

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
