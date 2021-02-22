# BN256

[![Build Status](https://travis-ci.org/clearmatics/bn256.svg?branch=master)](https://travis-ci.org/clearmatics/bn256)

This package implements a [particular](https://eprint.iacr.org/2013/507.pdf) bilinear group.
The code is imported from https://github.com/ethereum/go-ethereum/tree/master/crypto/bn256/cloudflare

:rotating_light: **WARNING** This package originally claimed to operate at a 128-bit level. However, [recent work](https://ellipticnews.wordpress.com/2016/05/02/kim-barbulescu-variant-of-the-number-field-sieve-to-compute-discrete-logarithms-in-finite-fields/) suggest that **this is no longer the case**.

## A note on the selection of the bilinear group

The parameters defined in the `constants.go` file follow the parameters used in [alt-bn128 (libff)](https://github.com/scipr-lab/libff/blob/master/libff/algebra/curves/alt_bn128/alt_bn128_init.cpp). These parameters were selected so that `r−1` has a high 2-adic order. This is key to improve efficiency of the key and proof generation algorithms of the SNARK used.

## Installation

    go get github.com/clearmatics/bn256

## Development

This project uses [go modules](https://github.com/golang/go/wiki/Modules).
If you develop in your `GOPATH` and use GO 1.11, make sure to run:
```bash
export GO111MODULE=on
```

In fact:
>  (Inside $GOPATH/src, for compatibility, the go command still runs in the old GOPATH mode, even if a go.mod is found.)
See: https://blog.golang.org/using-go-modules

> For more fine-grained control, the module support in Go 1.11 respects a temporary environment variable, GO111MODULE, which can be set to one of three string values: off, on, or auto (the default). If GO111MODULE=off, then the go command never uses the new module support. Instead it looks in vendor directories and GOPATH to find dependencies; we now refer to this as "GOPATH mode." If GO111MODULE=on, then the go command requires the use of modules, never consulting GOPATH. We refer to this as the command being module-aware or running in "module-aware mode". If GO111MODULE=auto or is unset, then the go command enables or disables module support based on the current directory. Module support is enabled only when the current directory is outside GOPATH/src and itself contains a go.mod file or is below a directory containing a go.mod file.
See: https://golang.org/cmd/go/#hdr-Preliminary_module_support

The project follows standard Go conventions using `gofmt`. If you wish to contribute to the project please follow standard Go conventions. The CI server automatically runs these checks.
