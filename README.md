# ipfs-update

> An updater tool for ipfs. Can fetch and install given versions of go-ipfs.

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

## Install

Requirement: Go version 1.5 or higher.

```sh
go get -u github.com/ipfs/ipfs-update
```


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

Downloads, tests, and installs the specified version
of ipfs. The existing version is stashed in case a revert is needed.

#### revert

`$ ipfs-update revert`

Reverts to the previously installed version of ipfs. This
is useful if the newly installed version has issues and you would like to switch
back to your older stable installation.

#### fetch

`$ ipfs-update fetch`

Downloads the specified version of ipfs into your current
directory. This is a plumbing command that can be utilized in scripts or by
more advanced users.

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/ipfs/ipfs-update/issues)!

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/contributing.md)

## License

[MIT](LICENSE)

