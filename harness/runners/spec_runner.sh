#!/usr/bin/env bash
set -euo pipefail

echo "[Harness Runner] Executing Spec BDD Scenarios and Invariant Assertions..."

# 1. 执行 Go 原生全套单元测试与 Harness 跑通验证
go test -race -v ./pkg/... ./testings/...

echo "[Harness Runner] All Spec BDD assertions completed successfully."
