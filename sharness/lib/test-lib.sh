# Test framework for ipfs-update
#
# Copyright (c) 2015 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#
# We are using Sharness (https://github.com/mlafeldt/sharness)
# which was extracted from the Git test framework.

SHARNESS_LIB="lib/sharness/sharness.sh"

# Set sharness verbosity. we set the env var directly as
# it's too late to pass in --verbose, and --verbose is harder
# to pass through in some cases.
test "$TEST_VERBOSE" = 1 && verbose=t && echo '# TEST_VERBOSE='"$TEST_VERBOSE"

. "$SHARNESS_LIB" || {
	echo >&2 "Cannot source: $SHARNESS_LIB"
	echo >&2 "Please check Sharness installation."
	exit 1
}

# Please put ipfs-update specific shell functions and variables below

DEFAULT_DOCKER_IMG="debian"
DOCKER_IMG="$DEFAULT_DOCKER_IMG"

TEST_TRASH_DIR=$(pwd)
TEST_SCRIPTS_DIR=$(dirname "$TEST_TRASH_DIR")
APP_ROOT_DIR=$(dirname "$TEST_SCRIPTS_DIR")

CERTIFS='/etc/ssl/certs:/etc/ssl/certs'

# This writes a docker ID on stdout
start_docker() {
	docker run -it -d -v "$CERTIFS" -v "$APP_ROOT_DIR:/mnt" -w "/mnt" "$DOCKER_IMG" /bin/bash
}

# This takes a docker ID and a command as arguments
exec_docker() {
	docker exec -i "$1" /bin/bash -c "$2"
}

# This takes a docker ID as argument
stop_docker() {
	docker stop "$1"
}

# Echo the args, run the cmd, and then also fail,
# making sure a test case fails.
test_fsh() {
    echo "> $@"
    eval "$@"
    echo ""
    false
}

test_install_version() {
	VERSION="$1"

	test_expect_success "'ipfs-update install' works for $VERSION" '
		exec_docker "$DOCID" "$GUEST_IPFS_UPDATE --verbose install --allow-downgrade $VERSION" >actual 2>&1 ||
		test_fsh cat actual
	'

	test_expect_success "'ipfs-update install' output looks good" '
		grep "fetching go-ipfs version $VERSION" actual &&
		grep "Installation complete." actual ||
		test_fsh cat actual
	'

	test_expect_success "'ipfs-update version' works for $VERSION" '
		exec_docker "$DOCID" "$GUEST_IPFS_UPDATE version" >actual
	'

	test_expect_success "'ipfs-update version' output looks good" '
		echo "$VERSION" >expected &&
		test_cmp expected actual
	'
}
