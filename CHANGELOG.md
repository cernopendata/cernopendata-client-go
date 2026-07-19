# Changelog

## [1.0.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.9.1...v1.0.0) (2026-07-19)


### ⚠ BREAKING CHANGES

* use github.com/cernopendata/cernopendata-client-go for Go installs and imports.

### Code Refactoring

* migrate module path to cernopendata ([7d2505b](https://github.com/cernopendata/cernopendata-client-go/commit/7d2505bd593095d1008e6444fd3d1122ac34429f))

## [0.9.1](https://github.com/cernopendata/cernopendata-client-go/compare/v0.9.0...v0.9.1) (2026-04-14)


### Bug Fixes

* **download:** preserve unique paths for duplicate basenames ([a37c42d](https://github.com/cernopendata/cernopendata-client-go/commit/a37c42d3f5e3070d0e8319976e66f5d87168e2c4))

## [0.9.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.8.1...v0.9.0) (2026-02-15)


### Features

* **cli:** restore JSON format output for get-file-locations and list-directory ([385151d](https://github.com/cernopendata/cernopendata-client-go/commit/385151dbd93995ebb1289161bd6033c7f24e5664))


### Bug Fixes

* **security:** address gosec findings in downloader/updater ([2d817c4](https://github.com/cernopendata/cernopendata-client-go/commit/2d817c4d94d360da2274159ae96e029feeb41ba5))

## [0.8.1](https://github.com/cernopendata/cernopendata-client-go/compare/v0.8.0...v0.8.1) (2026-02-04)


### Bug Fixes

* **cli:** change list-directory format short flag to -m for consistency ([a618edc](https://github.com/cernopendata/cernopendata-client-go/commit/a618edcad85cba8dc08b460545c74961f3ed94db))

## [0.8.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.7.2...v0.8.0) (2026-02-04)


### Features

* add JSON output to get-file-locations ([2a51004](https://github.com/cernopendata/cernopendata-client-go/commit/2a51004098a294b44d302e1da35fa7bf38af155f))
* add JSON output to list-directory ([2ae04e9](https://github.com/cernopendata/cernopendata-client-go/commit/2ae04e9e95472310b4ce1f5860b5fc7dcda9e356))


### Bug Fixes

* use dynamic map for complete metadata retrieval ([3812f42](https://github.com/cernopendata/cernopendata-client-go/commit/3812f422003d761efd4a53467e2237a25334cd24))

## [0.7.2](https://github.com/cernopendata/cernopendata-client-go/compare/v0.7.1...v0.7.2) (2026-02-04)


### Bug Fixes

* **xrootd:** tune read buffer ([e63f90f](https://github.com/cernopendata/cernopendata-client-go/commit/e63f90fc4cb9966dd88db1a3f9cf11a280926027))

## [0.7.1](https://github.com/cernopendata/cernopendata-client-go/compare/v0.7.0...v0.7.1) (2026-02-04)


### Bug Fixes

* **checksum:** stream adler32 hashing ([4d102c9](https://github.com/cernopendata/cernopendata-client-go/commit/4d102c9f90bd85b4bfe82d9b55d277988ec32a00))
* **progress:** throttle updates ([d7e2530](https://github.com/cernopendata/cernopendata-client-go/commit/d7e2530ce3fdb2ce00d40f16cc84c2f77bd7e9fe))
* **searcher:** add 30s http timeout ([95d4cb4](https://github.com/cernopendata/cernopendata-client-go/commit/95d4cb4d977ce63a5858d56a95d077690ebc9d9a))

## [0.7.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.6.0...v0.7.0) (2026-02-04)


### Features

* enable resuming of interrupted downloads ([4bfcc2c](https://github.com/cernopendata/cernopendata-client-go/commit/4bfcc2c44a2abdbe3d1d120ed248e62ecd1cd030))


### Bug Fixes

* **downloader:** strict resume on retry ([ec56bd1](https://github.com/cernopendata/cernopendata-client-go/commit/ec56bd129dbcac20428aeb44fa5aab8b0f71b548))

## [0.6.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.5.0...v0.6.0) (2026-02-04)


### Features

* **download:** add real-time progress display for file downloads ([c8dd5b2](https://github.com/cernopendata/cernopendata-client-go/commit/c8dd5b25ceef66d73862f2f7f8c6c5950e7c2f48))

## [0.5.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.4.1...v0.5.0) (2026-01-20)


### Features

* **cli:** implement --file-availability flag and enhance UX ([1ece55b](https://github.com/cernopendata/cernopendata-client-go/commit/1ece55bdc487a331973deccccf4ce92a039708c2))
* **searcher:** add file availability metadata and filtering ([65b2f73](https://github.com/cernopendata/cernopendata-client-go/commit/65b2f73f23a3279d2f5be34a416beba2b7b9114d))
* **utils:** add byte formatting utility ([f4972cc](https://github.com/cernopendata/cernopendata-client-go/commit/f4972cc0e3bc608797bab2018a9da29cd1f8658f))

## [0.4.1](https://github.com/cernopendata/cernopendata-client-go/compare/v0.4.0...v0.4.1) (2026-01-20)


### Bug Fixes

* resolve integration test linter warnings and update pre-commit config ([9003e42](https://github.com/cernopendata/cernopendata-client-go/commit/9003e42868f1e6c8b416139f362d593438ed51cb))

## [0.4.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.3.0...v0.4.0) (2026-01-19)


### Features

* **cli:** add update command with --check flag ([0379cb8](https://github.com/cernopendata/cernopendata-client-go/commit/0379cb8ef4e05e82e8f4b9679210548dcd90709f))
* **updater:** add self-update functionality with GitHub API ([78ce5df](https://github.com/cernopendata/cernopendata-client-go/commit/78ce5df90df8029789824c0d10966e673a7919e2))


### Bug Fixes

* remove duplicate TestIntegrationVersion function ([4235183](https://github.com/cernopendata/cernopendata-client-go/commit/42351830836533344410ec3217da296f5e5e3e0a))

## [0.3.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.2.0...v0.3.0) (2026-01-19)


### Features

* add installer script for easy binary installation ([b46b632](https://github.com/cernopendata/cernopendata-client-go/commit/b46b632f7c0ebe030c33d4398328350b0d2f7977))

## [0.2.0](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.5...v0.2.0) (2026-01-19)


### Features

* **cli:** add search command with flexible query options ([61b708b](https://github.com/cernopendata/cernopendata-client-go/commit/61b708bd305098b49f784017cbac86fded544db0)), closes [#4](https://github.com/cernopendata/cernopendata-client-go/issues/4)
* **searcher:** add SearchRecords and GetFacets methods ([2c6d7c9](https://github.com/cernopendata/cernopendata-client-go/commit/2c6d7c99c53c337d45972adbc770670c323f0de4))
* **utils:** add ParseQueryFromURL for URL/query string parsing ([8871414](https://github.com/cernopendata/cernopendata-client-go/commit/8871414775223902ffb274431763e1292f62bc9e))

## [0.1.5](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.4...v0.1.5) (2026-01-18)


### Bug Fixes

* **build:** correct buildVersion package path in LDFLAGS ([44e7d85](https://github.com/cernopendata/cernopendata-client-go/commit/44e7d8537a3cb903c81b5f311987c674689c6221))
* **tests:** increase timeout in TestIntegrationListDirectoryTimeout to avoid hanging ([1fe1783](https://github.com/cernopendata/cernopendata-client-go/commit/1fe1783bede686c2f7f189fbfabdad3916c64c27))

## [0.1.4](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.3...v0.1.4) (2026-01-18)


### Bug Fixes

* download release assets instead of rebuilding for checksums ([dba9df7](https://github.com/cernopendata/cernopendata-client-go/commit/dba9df78d821a98bc422b24bea22c986f1619fae))

## [0.1.3](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.2...v0.1.3) (2026-01-18)


### Bug Fixes

* separate checksum upload into dedicated job that waits for binaries ([feacaf7](https://github.com/cernopendata/cernopendata-client-go/commit/feacaf79b8b7c24e3e9839ddff7374aedd5fe98a))

## [0.1.2](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.1...v0.1.2) (2026-01-18)


### Bug Fixes

* build and upload only specific binary per matrix job to avoid conflicts ([1ac3d33](https://github.com/cernopendata/cernopendata-client-go/commit/1ac3d33b3e75e3632a8415ce9a47205b893d25d1))

## [0.1.1](https://github.com/cernopendata/cernopendata-client-go/compare/v0.1.0...v0.1.1) (2026-01-18)


### Bug Fixes

* avoid race condition when uploading checksums by handling in single job ([09efbe8](https://github.com/cernopendata/cernopendata-client-go/commit/09efbe875c493e1368c3c131860d8763e9f90be1))

## 0.1.0 (2026-01-18)


### Features

* add path normalization and fix XRootD compatibility ([dc0243d](https://github.com/cernopendata/cernopendata-client-go/commit/dc0243d6381e0a2eef1b6fecdadd65c470e896f9))
* align download-files command flags with Python client ([0c33d86](https://github.com/cernopendata/cernopendata-client-go/commit/0c33d861f2c8abcb80aebd4da245b065c86de44a))
* improve CLI help text and add --no-expand flag to match Python client ([fa5d5c9](https://github.com/cernopendata/cernopendata-client-go/commit/fa5d5c9a9f2309e45f67c8d369bc6e059f0546ab))
* improve test coverage and Python CLI compatibility ([126ed6f](https://github.com/cernopendata/cernopendata-client-go/commit/126ed6fb3a643ccbee514a670f77b6cb755b3482))
* initial implementation of CERN OpenData client in Go ([7560e46](https://github.com/cernopendata/cernopendata-client-go/commit/7560e46fc771799fb4f0162a7b99e72768608573))
* update list-directory command to match Python client ([e0e19eb](https://github.com/cernopendata/cernopendata-client-go/commit/e0e19ebb2064241b079459d1867c7ebd862402bd))


### Bug Fixes

* handle XRootD EOF detection and add context cancellation ([8cde4e1](https://github.com/cernopendata/cernopendata-client-go/commit/8cde4e1a605a25c475b390d99898b65b99f1d013))
* rename to github.com/cernopendata/cernopendata-client-go ([13b4933](https://github.com/cernopendata/cernopendata-client-go/commit/13b4933965e9ed6760159d89bec15ad72175b1ce))


### Miscellaneous Chores

* release 0.1.0 ([2d1ea72](https://github.com/cernopendata/cernopendata-client-go/commit/2d1ea72b0fa182a52801955117686f650684269b))

## Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
