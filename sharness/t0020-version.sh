#!/bin/sh

test_description="ipfs-update version"

. lib/test-lib.sh

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_expect_success "ipfs-update binary is there" '
	exec_docker "$DOCID" "test -f sharness/bin/ipfs-update"
'

test_expect_success "'ipfs-update version' works" '
	exec_docker "$DOCID" "sharness/bin/ipfs-update version" >actual
'

test_expect_success "'ipfs-update version' output looks good" '
	echo "none" >expected &&
	test_cmp expected actual
'

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
