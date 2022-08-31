[![GitHub release](https://img.shields.io/github/release/cybozu-go/nyamber.svg?maxAge=60)][releases]
[![CI](https://github.com/cybozu-go/nyamber/actions/workflows/ci.yaml/badge.svg)](https://github.com/cybozu-go/nyamber/actions/workflows/ci.yaml)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/cybozu-go/nyamber?tab=overview)](https://pkg.go.dev/github.com/cybozu-go/nyamber?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/nyamber)](https://goreportcard.com/report/github.com/cybozu-go/nyamber)

Nyamber
============================
Nyamber is a custom controller to create Neco environment

## Features
- Run a dctest in a pod
- Created a pod which runs dctest and a service to access the pod corresponding to vdc custom resources
- Run user-defined command
- Reflect to vdc resources in status of executed command

## Supported Software
- Kubernetes: 1.22, 1.23, 1.24

## Documentation

[docs](docs/) directory contains documents about designs and specifications.

[releases]: https://github.com/cybozu-go/nyamber/releases
