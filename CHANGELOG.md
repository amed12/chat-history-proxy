# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added `.gitignore` to prevent committing sensitive environment variables, docker configs, logs, and temporary files.
- Added `docker-compose.yml.example` as a template configuration for local deployment.
- Added connection string sanitizer utility `internal/security/redactor.go` to mask passwords in DSN/database URLs.
- Added unit tests for database URL masking function in `internal/security/redactor_test.go`.

### Changed
- Updated `internal/database/db.go` to mask database credentials in connection failure and ping failure logs.
- Removed `docker-compose.yml` from Git tracking to avoid committing local credentials.
