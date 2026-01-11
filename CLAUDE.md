# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go CLI tool that automates downloading invoices, bank statements, payslips, and other documents from various French service providers using browser automation (Playwright). Each provider is implemented as a separate internal package with its own web scraping logic.

## Development Commands

### Building
```bash
go build -o downloader .
```

### Running with Nix
This project uses Nix flakes for reproducible builds. Enter the development environment:
```bash
nix develop
```

The Nix flake automatically sets up:
- Playwright browser paths via `PLAYWRIGHT_BROWSERS_PATH`
- Playwright driver via `PLAYWRIGHT_DRIVER_PATH`
- Node.js path for Playwright via `PLAYWRIGHT_NODEJS_PATH`

Build the project with Nix:
```bash
nix build
```

### Linting
```bash
golangci-lint run
```

### Installing Playwright Browsers
Set environment variable and run:
```bash
export PLAYWRIGHT_INSTALL=1
./downloader <command> [flags]
```

## Architecture

### CLI Structure
- Uses `alecthomas/kong` for CLI parsing
- Main entry point: `main.go` defines all commands and global flags
- Each command (Freebox, FreeMobile, etc.) is a struct with a `Run(ctx *Context) error` method
- Context contains: `OutputDir`, `Headless`, `NoInteraction`

### Provider Pattern
Each provider lives in `internal/<provider>/` and implements:
1. A `Run()` function that accepts stdout/stderr, credentials, and options
2. Uses the shared `internal/pw` package for Playwright automation
3. Typical flow: login → navigate → download → save to output directory

### Playwright Utilities (`internal/pw/`)
The `pw` package provides:
- `Run()`: Main wrapper that handles Playwright lifecycle, browser launch, context creation
- Cookie persistence: Automatically saves/loads cookies from `cookies.json` for session reuse
- Screenshot on error: Saves to `screenshots/screenshot.png` when automation fails
- `Download()`: Helper to trigger and save file downloads
- `DownloadPDFPopup()`: Helper for handling PDF downloads in popup windows
- Browser selection: Supports Chromium (default) and Firefox

All provider implementations call `pw.Run()` with a callback containing their specific automation logic.

### Provider Implementations

Each provider in `internal/` follows this pattern:
- `Run()` function as public API
- `downloadFile()` orchestrates the workflow
- `login()` handles authentication
- `downloadAndSave()` handles file retrieval

Current providers:
- `freebox`: Freebox internet invoices
- `freemobile`: Free mobile phone invoices (supports `NoInteraction` flag)
- `eaudugrandlyon`: Water utility invoices
- `octopusenergyaddress`: Energy proof of address (marked non-functional in README)
- `shiva`: Payslips
- `lclchecking`: Bank statements from LCL
- `photos`: Photo downloads (no credentials required)

## Key Dependencies
- `playwright-community/playwright-go`: Browser automation
- `alecthomas/kong`: CLI argument parsing
- `carlmjohnson/requests`: HTTP client utilities

## Environment Setup
The Nix flake sets up Playwright environment variables automatically. Without Nix, you must manually set:
- `PLAYWRIGHT_DRIVER_PATH`: Path to Playwright driver
- `PLAYWRIGHT_BROWSERS_PATH`: Path to Playwright browsers
- `PLAYWRIGHT_NODEJS_PATH`: Path to Node.js binary
- `PLAYWRIGHT_INSTALL=1`: To install browsers on first run
