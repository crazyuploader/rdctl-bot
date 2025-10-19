#!/bin/bash
set -e

echo "=========================================="
echo "Running Unit Tests for Database Support"
echo "=========================================="
echo ""

echo "→ Running tests for internal/config..."
go test -v ./internal/config

echo ""
echo "→ Running tests for internal/db..."
go test -v ./internal/db

echo ""
echo "→ Running tests for internal/bot..."
go test -v ./internal/bot

echo ""
echo "=========================================="
echo "All Tests Passed! ✓"
echo "=========================================="
echo ""

echo "→ Running coverage analysis..."
go test -cover ./internal/config ./internal/db ./internal/bot

echo ""
echo "For detailed coverage report, run:"
echo "  go test -coverprofile=coverage.out ./..."
echo "  go tool cover -html=coverage.out"