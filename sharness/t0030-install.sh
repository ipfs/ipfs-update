#!/bin/sh

test_description="ipfs-update install"

. lib/test-lib.sh

GUEST_IPFS_UPDATE="sharness/bin/ipfs-update"

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_expect_success "'ipfs-update install' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE install v0.3.9" >actual
'

test_expect_success "'ipfs-update install' output looks good" '
	grep "installing ipfs version v0.3.9" actual
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
