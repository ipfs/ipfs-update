# ipfs-update

> An updater tool for ipfs. Can fetch and install given versions of go-ipfs.

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![Travis CI](https://travis-ci.org/ipfs/ipfs-update.svg?branch=master)](https://travis-ci.org/ipfs/ipfs-update)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

## Install

You can either install a pre-built binary or build `ipfs-update` from source.

### Pre-built Binaries

You can download pre-built binaries at: https://dist.ipfs.io/#ipfs-update

### From Source

`ipfs-update` uses go modules and requires Go version 1.12 or higher:

```sh
# You need to CD to a directory _outside_ of your GOPATH.
$ cd /
# Install with go modules enabled
$ GO111MODULE=on go get github.com/ipfs/ipfs-update
```

Note: Your $GOPATH/bin should be within $PATH for the result ipfs-update binary
to be found.

## Usage

If you do not see the expected version listed by `ipfs-update versions`. Try updating
`ipfs-update` (either by the above `go get` command or through gobuilder).

#### version

`$ ipfs-update version`

Prints out the version of ipfs that is currently installed.

#### versions

`$ ipfs-update versions`

Prints out all versions of ipfs available for installation.

#### install

`$ ipfs-update install <version>`

Downloads, tests, and installs the specified version (or "latest" for
latest version) of ipfs. The existing version is stashed in case a revert is needed.

#### revert

`$ ipfs-update revert`

Reverts to the previously installed version of ipfs. This
is useful if the newly installed version has issues and you would like to switch
back to your older stable installation.

#### fetch

`$ ipfs-update fetch [version]`

Downloads the specified version of ipfs into your current
directory. This is a plumbing command that can be utilized in scripts or by
more advanced users.

## Install Location

`ipfs-update` tries to intelligently pick the correct install location for
go-ipfs.

1. If you have go-ipfs installed, `ipfs-update` will install over your existing install.
2. If you have a Go development environment setup, it will install go-ipfs along
   with all of your other go programs.
3. Otherwise, it will try to pick a sane, writable install location.

Specifically, `ipfs-update` will install go-ipfs according to the following
algorithm:

0. If `go-ipfs` is already installed and in your PATH, `ipfs-update` will
   replace it. `ipfs-update` will _fail_ if it can't and won't try to install
   elsewhere.
1. If Go is installed:
  1. [GOBIN][go-env] if GOBIN is in your PATH.
  2. For each `$path` in GOPATH, `$path/bin` if it's in your PATH.
2. On Windows:
  1. The current directory if it's writable and in your PATH.
  2. The directory where the ipfs-update executable lives if it's executable and in your path.
  3. The directory where the ipfs-update executable lives if it's executable and in your current working directory.
3. On all platforms _except_ Windows:
  1. If root:
    1. `/usr/local/bin` if it exists, is writable, and is in your PATH.
    2. `/usr/bin` if it exists, is writable, and is in your PATH.
  2. `$HOME/.local/bin` if it exists, is writable, and is in your PATH.
  3. `$HOME/bin` if it exists, is writable, and is in your PATH.
  4. `$HOME/.local/bin` if we can create it and it's in your PATH.
  5. `$HOME/bin` if we can create it and it's in your PATH.

[go-env]: https://golang.org/cmd/go/#hdr-Environment_variables

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/ipfs/ipfs-update/issues)!

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/contributing.md)

## License

[MIT](LICENSE)

