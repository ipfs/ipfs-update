#!/bin/sh

test_description="ipfs-update install"

. lib/test-lib.sh

GUEST_IPFS_UPDATE="sharness/bin/ipfs-update"

# TODO: remove this when migrations are availabe on the distributions site
IPFS_DIST_PATH="/ipfs/QmZqzacg5Q8WpDL7SymogoaSZYw6RZT2kgPavymCmMwWse"

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_expect_success "'ipfs-update install' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --verbose install v0.3.9" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success "'ipfs-update install' output looks good" '
	grep "fetching go-ipfs version v0.3.9" actual
'

test_expect_success "'ipfs-update version' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE version" >actual
'

test_expect_success "'ipfs-update version' output looks good" '
	echo "v0.3.9" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs-update install' works when something is installed" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --distpath $IPFS_DIST_PATH --verbose install v0.4.23" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success  "'ipfs-update install' fails when downgrading without the downgrade flag" '
	test_must_fail exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --verbose install v0.3.8" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success "'ipfs-update install' works when downgrading with flag" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --distpath $IPFS_DIST_PATH --verbose install --allow-downgrade v0.3.8" >actual 2>&1 ||
	test_fsh cat actual
'

test_expect_success "'ipfs-update install' output looks good" '
	grep "fetching go-ipfs version v0.3.8" actual
'

test_expect_success "'ipfs-update version' works" '
	exec_docker "$DOCID" "$GUEST_IPFS_UPDATE version" >actual
'

test_expect_success "'ipfs-update version' output looks good" '
	echo "v0.3.8" >expected &&
	test_cmp expected actual
'

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
