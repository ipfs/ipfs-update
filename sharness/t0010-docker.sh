#!/bin/sh

test_description="Basic Docker Tests"

. lib/test-lib.sh

test_expect_success "docker is installed" '
	type docker
'

test_expect_success "'docker --version' works" '
	docker --version >actual
'

test_expect_success "'docker --version' output looks good" '
	egrep "^Docker version" actual
'

test_expect_success "current user is in the 'docker' group" '
	groups | egrep "\bdocker\b"
'

test_expect_success "start a docker container" '
	DOCID=$(start_docker)
'

test_expect_success "exec a command in docker container" '
	exec_docker "$DOCID" "echo \"Hello world!\"" >actual
'

test_expect_success "command output looks good" '
	echo "Hello world!" >expected &&
	test_cmp expected actual
'

test_expect_success "stop a docker container" '
	stop_docker "$DOCID"
'

test_done
