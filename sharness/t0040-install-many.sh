#!/bin/sh

test_description="ipfs-update install with many different versions"

. lib/test-lib.sh

GUEST_IPFS_UPDATE="sharness/bin/ipfs-update"

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_install_version "v0.3.7"
test_install_version "v0.3.10"
test_install_version "v0.3.8"
test_install_version "v0.4.0"

test_expect_success "'ipfs init' succeeds" '
	exec_docker "$DOCID" "IPFS_PATH=/root/.ipfs BITS=2048 ipfs init" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success ".ipfs/ has been created" '
	exec_docker "$DOCID" "test -d  /root/.ipfs && test -f /root/.ipfs/config"
	exec_docker "$DOCID" "test -d  /root/.ipfs/datastore && test -d /root/.ipfs/blocks"
'

test_install_version "v0.3.10"

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
