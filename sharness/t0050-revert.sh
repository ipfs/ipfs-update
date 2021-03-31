#!/bin/sh

test_description="ipfs-update revert"

. lib/test-lib.sh

GUEST_IPFS_UPDATE="sharness/bin/ipfs-update"

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_install_version "v0.3.9"
test_install_version "v0.3.10"


test_expect_success "'ipfs-update revert' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --verbose revert" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success "'ipfs-update revert' output looks good" '
	grep "Reverting to" actual &&
	grep "Revert complete." actual ||
	test_fsh cat actual
'

test_expect_success "'ipfs-update version' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE version" >actual
'

test_expect_success "'ipfs-update version' output looks good" '
	echo "v0.3.9" >expected &&
	test_cmp expected actual
'

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
