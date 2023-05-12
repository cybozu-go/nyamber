# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [0.3.0] - 2023-05-12

### Changed

- Support Kubernetes 1.26 ([#47](https://github.com/cybozu-go/nyamber/pull/47))
- Build with go 1.20 ([#47](https://github.com/cybozu-go/nyamber/pull/47))

## [0.2.0] - 2023-02-07

### Changed

- Support Kubernetes 1.25 ([#43](https://github.com/cybozu-go/nyamber/pull/43))
- Build with go 1.19 ([#44](https://github.com/cybozu-go/nyamber/pull/44))
- Update Ubuntu base image ([#45](https://github.com/cybozu-go/nyamber/pull/45))

## [0.1.5] - 2023-01-27

This release updates the base image and the packages used by rebuilding the image.
This fixes git vulnerabilities
- USN-5810-1: Git vulnerabilities
- USN-5810-2: Git regression

## [0.1.4] - 2022-12-15
### Fix
- Refer avdc creationTimestamp when next start time is nil (#39)
- Validate avdc template when startSchedule is not set (#40)

## [0.1.3] - 2022-10-19
### Fix
- Replace CMD to ENTRYPOINT in Dockerfile.Runner (#33)
- Defaulting by controller instead admission webhook (#35)
### Added
- Add shorthand `avdc` (#34)

## [0.1.2] - 2022-09-28
### Fix
- Fix config (#32)
- Inherit pod metadata of pod_template (#33)

## [0.1.1] - 2022-09-26
### Removed
- kube-rbac-proxy is removed (#30)

## [0.1.0] - 2022-09-16

### Added

- This is the first public release.

[Unreleased]: https://github.com/cybozu-go/nyamber/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/cybozu-go/nyamber/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/cybozu-go/nyamber/compare/v0.1.5...v0.2.0
[0.1.5]: https://github.com/cybozu-go/nyamber/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/cybozu-go/nyamber/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/cybozu-go/nyamber/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/cybozu-go/nyamber/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/cybozu-go/nyamber/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/cybozu-go/nyamber/compare/0b95ddf1810b156fc2bd36edd457b96a18ca0501...v0.1.0
