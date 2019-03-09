# toolman

`toolman` helps [tracking tool dependencies for a Go module][track-tool-dep].

## Installation

```sh
go get -u github.com/execjosh/toolman
```

## Usage

To initialize a new `tools.go` with:

```sh
toolman -init
```

To start tracking a tool with:

```sh
toolman -add github.com/google/wire/cmd/wire
go mod tidy
```

To install all of the tools from a `tools.go`:

```sh
toolman
```

[track-tool-dep]: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module