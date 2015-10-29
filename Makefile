all: build

build:
	GO15VENDOREXPERIMENT=1 go build

install:
	GO15VENDOREXPERIMENT=1 go install
