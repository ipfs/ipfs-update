# ipfs update
An updater tool for ipfs. Can fetch and install given versions of go-ipfs.

## Installation

Requirement: Go version 1.5 or higher

```
go get -u github.com/ipfs/ipfs-update
```

## Usage

### `version`
`ipfs-update version` prints out the version of ipfs that is currently installed.

### `versions`
`ipfs-update versions` prints out all versions of ipfs available for installation.

### `install`
`ipfs-update install <version>` downloads, tests, and installs the specified version
of ipfs. The existing version is stashed in case a revert is needed.

### `revert`
`ipfs-update revert` reverts to the previously installed version of ipfs. This
is useful if the newly installed version has issues and you would like to switch
back to your older stable installation.

### `fetch`
`ipfs-update fetch` downloads the specified version of ipfs into your current
directory. This is a plumbing command that can be utilized in scripts or by
more advanced users.
