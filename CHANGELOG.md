# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- event recorder output to keep errors closer to the resource
### Fixed
- bug where no error would be produced if a key didn't exist in a referenced secret/configmap
- CRD definition stopping updates of status

## [1.0.0] - 2020-11-16
### Added
- Example [Connector](./manifests/examples/example-connector.yaml) 
- Changelog