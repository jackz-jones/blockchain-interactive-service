#!/usr/bin/env bash
# pre_push_check.sh — Universal pre-push checks (allows longer execution time)
#
# Design principles:
#   - Language-agnostic: auto-detects project type from manifest files
#   - AI-friendly: structured error output with full compiler/linter output
#   - Comprehensive: build + lint + test for changed packages
#   - Only blocks on real failures (new dependency warnings are non-blocking)
#
# Supported languages:
#   - Go (go.mod)        → go build, golangci-lint/staticcheck, go test
#   - Rust (Cargo.toml)  → cargo build/check, cargo clippy, cargo test
#   - TypeScript/JS (package.json) → tsc, eslint/biome, jest/vitest/mocha
#   - Python (pyproject.toml/setup.py) → mypy/pyright, ruff/flake8, pytest
#
# Skip: SKIP_PRE_PUSH=1 git push

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
RESET='\033[0m'

# ── AI-friendly output helpers ──────────────────────────────────
error()    { echo -e "${RED}[BLOCK]${RESET} $*" >&2; }
warn()     { echo -e "${YELLOW}[WARN]${RESET} $*" >&2; }
info()     { echo -e "${CYAN}[INFO]${RESET} $*"; }
section()  { echo -e "\n${CYAN}══ $* ══${RESET}"; }

# Print full command output for AI consumption
show_output() {
  local label="$1" file="$2"
  if [[ -s "$file" ]]; then
    echo -e "\n${RED}── ${label} output ──${RESET}" >&2
    cat "$file" >&2
    echo -e "${RED}── end of ${label} output ──${RESET}\n" >&2
  fi
}

# ── Global skip ─────────────────────────────────────────────────
if [[ "${SKIP_PRE_PUSH:-}" == "1" ]]; then
  echo -e "${YELLOW}[SKIP]${RESET} SKIP_PRE_PUSH=1, skipping all pre-push checks"
  exit 0
fi

# ── Environment setup ───────────────────────────────────────────
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR

REAL_SCRIPT="$(readlink "${BASH_SOURCE[0]}" 2>/dev/null || true)"
[[ -z "${REAL_SCRIPT}" ]] && REAL_SCRIPT="${BASH_SOURCE[0]}"
if [[ "${REAL_SCRIPT}" != /* ]]; then
  if [[ "${REAL_SCRIPT}" == "${BASH_SOURCE[0]}" ]]; then
    REAL_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${REAL_SCRIPT}")"
  else
    REAL_SCRIPT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && cd "$(dirname "${REAL_SCRIPT}")" && pwd)/$(basename "${REAL_SCRIPT}")"
  fi
fi
SCRIPT_DIR="$(cd "$(dirname "${REAL_SCRIPT}")" && pwd)"
PROJECT_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_ROOT}"

if [[ -f "${SCRIPT_DIR}/env.sh" ]]; then
  # shellcheck disable=SC1091
  source "${SCRIPT_DIR}/env.sh"
fi

# 目标分支优先使用项目配置；默认远端 HEAD 必须存在，不能猜历史提交。
# scripts/env.sh 的 SOP_TARGET_BRANCH 随仓库共享，Git 配置用于本地覆盖未配置的项目。
target_ref() {
  local target
  target="${SOP_TARGET_BRANCH:-$(git config --get sop.targetBranch || true)}"
  if [[ -n "$target" ]]; then
    git rev-parse --verify "refs/remotes/origin/$target^{commit}"
  else
    git rev-parse --verify 'refs/remotes/origin/HEAD^{commit}'
  fi
}
PUSH_FILES=()
REFS_SEEN=0
DIFF_BASE=""
while read -r local_ref local_sha remote_ref remote_sha; do
  REFS_SEEN=1
  [[ "$local_sha" == 0000000000000000000000000000000000000000 ]] && continue
  if [[ "$remote_sha" == 0000000000000000000000000000000000000000 ]]; then
    DIFF_BASE="$(target_ref)" || { error "无法确定目标分支，请配置 sop.targetBranch 并获取远端引用"; exit 1; }
    DIFF_BASE="$(git merge-base "$DIFF_BASE" "$local_sha")" || exit 1
  else
    # amend/force-with-lease 也按两个树比较，不能只看共同祖先之后的新增。
    DIFF_BASE="$remote_sha"
  fi
  files="$(git diff --name-only "$DIFF_BASE" "$local_sha")" || exit 1
  while IFS= read -r f; do
    [[ -n "$f" ]] && PUSH_FILES+=("$f")
  done <<< "$files"
done

# 手工检查没有 pre-push stdin 时，按项目目标分支检查当前 HEAD。
if [[ "$REFS_SEEN" -eq 0 ]]; then
  DIFF_BASE="$(target_ref)" || { error "无法确定目标分支，请配置 sop.targetBranch"; exit 1; }
  DIFF_BASE="$(git merge-base "$DIFF_BASE" HEAD)" || exit 1
  files="$(git diff --name-only "$DIFF_BASE" HEAD)" || exit 1
  while IFS= read -r f; do
    [[ -n "$f" ]] && PUSH_FILES+=("$f")
  done <<< "$files"
fi
if [[ ${#PUSH_FILES[@]} -eq 0 ]]; then
  info "没有需要检查的文件"
  exit 0
fi

# 仓库存在语言清单并不代表本次修改了该语言，例如 Go 仓库也可能有 ruff 配置。
HAS_GO=0; HAS_RUST=0; HAS_TS=0; HAS_JS=0; HAS_PYTHON=0; HAS_JAVA=0
for f in "${PUSH_FILES[@]}"; do
  case "$f" in
    *.go|go.mod|go.sum|go.work|go.work.sum|.golangci.*) HAS_GO=1 ;;
    *.rs|Cargo.toml|Cargo.lock|rust-toolchain*) HAS_RUST=1 ;;
    *.ts|*.tsx|tsconfig*.json) HAS_TS=1 ;;
    *.js|*.jsx|*.mjs|*.cjs|package.json|package-lock.json|yarn.lock|pnpm-lock.yaml) HAS_JS=1 ;;
    *.py|*.pyi|pyproject.toml|setup.py|setup.cfg|requirements*.txt|pytest.ini|tox.ini|ruff.toml|.ruff.toml) HAS_PYTHON=1 ;;
    *.java|*.kt|*.kts|pom.xml|gradle.properties) HAS_JAVA=1 ;;
    Makefile|*.mk|scripts/pre_push_check.sh|scripts/env.sh)
      [[ -f go.mod ]] && HAS_GO=1
      [[ -f Cargo.toml ]] && HAS_RUST=1
      [[ -f package.json ]] && HAS_JS=1
      ;;
  esac
done

FAILED=0
CHECKS_RUN=0

# ══════════════════════════════════════════════════════════════════
# Go checks
# ══════════════════════════════════════════════════════════════════
run_go_checks() {
  [[ "${HAS_GO}" -eq 1 ]] || return 0
  command -v go >/dev/null 2>&1 || { warn "Go detected but 'go' not in PATH, skipping Go checks"; return 0; }

  export GOCACHE="${GOCACHE:-/tmp/go-build}"

  # --- Build ---
  section "Go: build"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local output; output=$(mktemp)
  if ! go build ./... >"$output" 2>&1; then
    error "go build ./... FAILED"
    show_output "go build" "$output"
    echo -e "  ${CYAN}Action:${RESET} Fix compilation errors shown above, then retry push." >&2
    FAILED=1
  else
    info "go build ./... passed"
  fi
  rm -f "$output"

  # --- Lint ---
  section "Go: lint"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)
  if command -v golangci-lint >/dev/null 2>&1; then
    if ! golangci-lint run --allow-serial-runners --new-from-rev="$DIFF_BASE" ./... >"$output" 2>&1; then
      error "golangci-lint run ./... FAILED"
      show_output "golangci-lint" "$output"
      echo -e "  ${CYAN}Action:${RESET} Fix lint issues above. Run 'golangci-lint run ./...' locally to verify." >&2
      FAILED=1
    else
      info "golangci-lint passed"
    fi
  elif command -v staticcheck >/dev/null 2>&1; then
    if ! staticcheck ./... >"$output" 2>&1; then
      error "staticcheck ./... FAILED"
      show_output "staticcheck" "$output"
      FAILED=1
    else
      info "staticcheck passed"
    fi
  elif [[ -f "Makefile" ]] && grep -q "^lint:" Makefile 2>/dev/null; then
    if ! make lint >"$output" 2>&1; then
      error "make lint FAILED"
      show_output "make lint" "$output"
      FAILED=1
    else
      info "make lint passed"
    fi
  else
    warn "No Go linter found (golangci-lint, staticcheck, or make lint). Skipping lint."
  fi
  rm -f "$output"

  # --- Test (affected packages only) ---
  section "Go: test (affected packages)"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local test_pkgs=()
  for f in "${PUSH_FILES[@]}"; do
    [[ "$f" =~ \.go$ ]] || continue
    local dir; dir="$(dirname "$f")"
    # Skip if directory doesn't exist (deleted files)
    [[ -d "$dir" ]] || continue
    [[ "$dir" == it || "$dir" == it/* ]] && continue
    # Check if package has test files
    if ls "${dir}"/*_test.go >/dev/null 2>&1; then
      # Deduplicate
      local already=0
      for existing in "${test_pkgs[@]+"${test_pkgs[@]}"}"; do
        [[ "$existing" == "./$dir" ]] && { already=1; break; }
      done
      [[ "$already" -eq 0 ]] && test_pkgs+=("./$dir")
    fi
  done

  if [[ ${#test_pkgs[@]} -gt 0 ]]; then
    output=$(mktemp)
    local failed_pkgs=()
    for pkg in "${test_pkgs[@]}"; do
      # 组件 IT 只编译，避免 TestMain 或场景访问尚未分配的环境。
      local test_args=(-count=1 -timeout 60s)
      local binary=""
      if [[ "$pkg" == ./it/* ]]; then
        binary="$(mktemp)"
        test_args=(-c -tags=it -o "$binary")
      fi
      if ! go test "${test_args[@]}" "$pkg" >"$output" 2>&1; then
        failed_pkgs+=("$pkg")
        error "go test ${pkg} FAILED"
        show_output "go test ${pkg}" "$output"
      fi
      [[ -z "$binary" ]] || rm -f "$binary"
    done
    rm -f "$output"
    if [[ ${#failed_pkgs[@]} -gt 0 ]]; then
      error "Go tests failed: ${#failed_pkgs[@]}/${#test_pkgs[@]} packages"
      echo -e "  ${CYAN}Failed packages:${RESET}" >&2
      printf "    %s\n" "${failed_pkgs[@]}" >&2
      echo -e "  ${CYAN}Action:${RESET} Fix test failures, then retry push." >&2
      FAILED=1
    else
      info "Go tests passed (${#test_pkgs[@]} packages)"
    fi
  else
    info "No Go test packages affected"
  fi
}

# ══════════════════════════════════════════════════════════════════
# Rust checks
# ══════════════════════════════════════════════════════════════════
run_rust_checks() {
  [[ "${HAS_RUST}" -eq 1 ]] || return 0
  command -v cargo >/dev/null 2>&1 || { warn "Rust detected but 'cargo' not in PATH, skipping Rust checks"; return 0; }

  # --- Build/Check ---
  section "Rust: check"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local output; output=$(mktemp)
  if ! cargo check --all-targets 2>"$output"; then
    error "cargo check --all-targets FAILED"
    show_output "cargo check" "$output"
    echo -e "  ${CYAN}Action:${RESET} Fix compilation errors shown above." >&2
    FAILED=1
  else
    info "cargo check passed"
  fi
  rm -f "$output"

  # --- Clippy (lint) ---
  section "Rust: clippy"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)
  if command -v cargo-clippy >/dev/null 2>&1 || cargo clippy --version >/dev/null 2>&1; then
    if ! cargo clippy --all-targets -- -D warnings 2>"$output"; then
      error "cargo clippy FAILED"
      show_output "cargo clippy" "$output"
      echo -e "  ${CYAN}Action:${RESET} Fix clippy warnings. Run 'cargo clippy --all-targets -- -D warnings' locally." >&2
      FAILED=1
    else
      info "cargo clippy passed"
    fi
  else
    warn "cargo-clippy not installed, skipping lint"
  fi
  rm -f "$output"

  # --- Test ---
  section "Rust: test"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)
  if ! cargo test 2>"$output"; then
    error "cargo test FAILED"
    show_output "cargo test" "$output"
    echo -e "  ${CYAN}Action:${RESET} Fix test failures. Run 'cargo test' locally to reproduce." >&2
    FAILED=1
  else
    info "cargo test passed"
  fi
  rm -f "$output"
}

# ══════════════════════════════════════════════════════════════════
# TypeScript/JavaScript checks
# ══════════════════════════════════════════════════════════════════
run_ts_js_checks() {
  [[ "${HAS_TS}" -eq 1 || "${HAS_JS}" -eq 1 ]] || return 0
  [[ -f "package.json" ]] || return 0

  # Detect package manager
  local pm="npm"
  [[ -f "pnpm-lock.yaml" ]] && pm="pnpm"
  [[ -f "yarn.lock" ]] && pm="yarn"
  [[ -f "bun.lockb" || -f "bun.lock" ]] && pm="bun"

  local run_cmd="${pm} run"
  [[ "$pm" == "npm" ]] && run_cmd="npx"

  # --- TypeScript compilation ---
  if [[ "${HAS_TS}" -eq 1 ]] && [[ -f "tsconfig.json" ]]; then
    section "TypeScript: type check"
    CHECKS_RUN=$((CHECKS_RUN + 1))
    local output; output=$(mktemp)
    if command -v tsc >/dev/null 2>&1 || npx tsc --version >/dev/null 2>&1; then
      if ! npx tsc --noEmit >"$output" 2>&1; then
        error "tsc --noEmit FAILED (TypeScript type errors)"
        show_output "tsc" "$output"
        echo -e "  ${CYAN}Action:${RESET} Fix TypeScript type errors shown above." >&2
        FAILED=1
      else
        info "tsc --noEmit passed"
      fi
    fi
    rm -f "$output"
  fi

  # --- Lint ---
  section "TypeScript/JavaScript: lint"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local output; output=$(mktemp)
  local lint_ran=0

  # Try biome first, then eslint
  if [[ -f "biome.json" || -f "biome.jsonc" ]] && command -v biome >/dev/null 2>&1; then
    if ! biome check . >"$output" 2>&1; then
      error "biome check FAILED"
      show_output "biome check" "$output"
      echo -e "  ${CYAN}Fix:${RESET} biome check --write ." >&2
      FAILED=1
    else
      info "biome check passed"
    fi
    lint_ran=1
  elif [[ -f ".eslintrc" || -f ".eslintrc.js" || -f ".eslintrc.json" || -f ".eslintrc.yml" || -f "eslint.config.js" || -f "eslint.config.mjs" || -f "eslint.config.ts" ]]; then
    if ! npx eslint . >"$output" 2>&1; then
      error "eslint FAILED"
      show_output "eslint" "$output"
      echo -e "  ${CYAN}Fix:${RESET} npx eslint . --fix" >&2
      FAILED=1
    else
      info "eslint passed"
    fi
    lint_ran=1
  fi

  if [[ "$lint_ran" -eq 0 ]]; then
    # Check if there's a lint script in package.json
    if grep -q '"lint"' package.json 2>/dev/null; then
      if ! ${pm} run lint >"$output" 2>&1; then
        error "${pm} run lint FAILED"
        show_output "${pm} run lint" "$output"
        FAILED=1
      else
        info "${pm} run lint passed"
      fi
    else
      warn "No linter configuration found, skipping lint"
    fi
  fi
  rm -f "$output"

  # --- Test ---
  section "TypeScript/JavaScript: test"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)
  if grep -q '"test"' package.json 2>/dev/null; then
    # Check if test script is a placeholder ("test": "echo \"Error: no test specified\" && exit 1")
    local test_script
    test_script=$(grep '"test"' package.json | head -1)
    if echo "$test_script" | grep -q "no test specified"; then
      info "No test script configured, skipping"
    else
      if ! ${pm} run test >"$output" 2>&1; then
        error "${pm} run test FAILED"
        show_output "${pm} run test" "$output"
        echo -e "  ${CYAN}Action:${RESET} Fix test failures. Run '${pm} run test' locally." >&2
        FAILED=1
      else
        info "${pm} run test passed"
      fi
    fi
  else
    info "No test script in package.json, skipping"
  fi
  rm -f "$output"
}

# ══════════════════════════════════════════════════════════════════
# Python checks
# ══════════════════════════════════════════════════════════════════
run_python_checks() {
  [[ "${HAS_PYTHON}" -eq 1 ]] || return 0

  # --- Type checking ---
  section "Python: type check"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local output; output=$(mktemp)
  local typecheck_ran=0

  if command -v mypy >/dev/null 2>&1 && [[ -f "mypy.ini" || -f "setup.cfg" || -f "pyproject.toml" ]]; then
    if ! mypy . >"$output" 2>&1; then
      error "mypy FAILED"
      show_output "mypy" "$output"
      echo -e "  ${CYAN}Action:${RESET} Fix type errors shown above." >&2
      FAILED=1
    else
      info "mypy passed"
    fi
    typecheck_ran=1
  elif command -v pyright >/dev/null 2>&1; then
    if ! pyright >"$output" 2>&1; then
      error "pyright FAILED"
      show_output "pyright" "$output"
      FAILED=1
    else
      info "pyright passed"
    fi
    typecheck_ran=1
  fi

  [[ "$typecheck_ran" -eq 0 ]] && info "No type checker (mypy/pyright) configured, skipping"
  rm -f "$output"

  # --- Lint ---
  section "Python: lint"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)
  local lint_ran=0

  if command -v ruff >/dev/null 2>&1; then
    if ! ruff check . >"$output" 2>&1; then
      error "ruff check FAILED"
      show_output "ruff check" "$output"
      echo -e "  ${CYAN}Fix:${RESET} ruff check --fix ." >&2
      FAILED=1
    else
      info "ruff check passed"
    fi
    lint_ran=1
  elif command -v flake8 >/dev/null 2>&1; then
    if ! flake8 . >"$output" 2>&1; then
      error "flake8 FAILED"
      show_output "flake8" "$output"
      FAILED=1
    else
      info "flake8 passed"
    fi
    lint_ran=1
  fi

  [[ "$lint_ran" -eq 0 ]] && warn "No Python linter (ruff/flake8) found, skipping"
  rm -f "$output"

  # --- Test ---
  section "Python: test"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  output=$(mktemp)

  if command -v pytest >/dev/null 2>&1; then
    local pytest_status=0
    pytest --tb=short >"$output" 2>&1 || pytest_status=$?
    if [[ "$pytest_status" -eq 5 ]]; then
      info "pytest 未发现测试用例；源码检查仍按结果执行"
    elif [[ "$pytest_status" -ne 0 ]]; then
      error "pytest FAILED"
      show_output "pytest" "$output"
      echo -e "  ${CYAN}Action:${RESET} Fix test failures. Run 'pytest --tb=long' for full tracebacks." >&2
      FAILED=1
    else
      info "pytest passed"
    fi
  elif [[ -f "Makefile" ]] && grep -q "^test:" Makefile 2>/dev/null; then
    if ! make test >"$output" 2>&1; then
      error "make test FAILED"
      show_output "make test" "$output"
      FAILED=1
    else
      info "make test passed"
    fi
  else
    info "No test runner (pytest) found, skipping"
  fi
  rm -f "$output"
}

# ══════════════════════════════════════════════════════════════════
# Java/Kotlin checks
# ══════════════════════════════════════════════════════════════════
run_java_checks() {
  [[ "${HAS_JAVA}" -eq 1 ]] || return 0

  section "Java/Kotlin: build & test"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local output; output=$(mktemp)

  if [[ -f "gradlew" ]]; then
    if ! ./gradlew build >"$output" 2>&1; then
      error "./gradlew build FAILED"
      show_output "gradle build" "$output"
      echo -e "  ${CYAN}Action:${RESET} Fix build/test errors. Run './gradlew build' locally." >&2
      FAILED=1
    else
      info "gradle build passed"
    fi
  elif [[ -f "mvnw" ]]; then
    if ! ./mvnw verify >"$output" 2>&1; then
      error "./mvnw verify FAILED"
      show_output "maven verify" "$output"
      FAILED=1
    else
      info "maven verify passed"
    fi
  elif command -v gradle >/dev/null 2>&1 && [[ -f "build.gradle" || -f "build.gradle.kts" ]]; then
    if ! gradle build >"$output" 2>&1; then
      error "gradle build FAILED"
      show_output "gradle build" "$output"
      FAILED=1
    else
      info "gradle build passed"
    fi
  elif command -v mvn >/dev/null 2>&1 && [[ -f "pom.xml" ]]; then
    if ! mvn verify >"$output" 2>&1; then
      error "mvn verify FAILED"
      show_output "maven verify" "$output"
      FAILED=1
    else
      info "maven verify passed"
    fi
  else
    warn "No Java build tool found (gradle/maven), skipping"
  fi
  rm -f "$output"
}

# ══════════════════════════════════════════════════════════════════
# Dependency change warning (non-blocking)
# ══════════════════════════════════════════════════════════════════
check_dependency_changes() {
  local base_ref
  base_ref="$DIFF_BASE"

  for f in "${PUSH_FILES[@]}"; do
    case "$f" in
      go.mod|Cargo.toml|package.json|pyproject.toml|requirements*.txt|pom.xml|build.gradle*)
        local new_deps
        new_deps=$(git diff "${base_ref}..HEAD" -- "$f" 2>/dev/null | grep -E '^[+]' | grep -v '^+++' | head -20 || true)
        if [[ -n "$new_deps" ]]; then
          warn "Dependency file '${f}' has changes. Ensure new dependencies are justified in PR description:"
          echo "$new_deps" | head -10 | sed 's/^/  /'
          echo ""
        fi
        ;;
    esac
  done
}

# ══════════════════════════════════════════════════════════════════
# Run all language-specific checks
# ══════════════════════════════════════════════════════════════════

info "Detected languages: $(
  [[ "${HAS_GO}" -eq 1 ]] && printf "Go "
  [[ "${HAS_RUST}" -eq 1 ]] && printf "Rust "
  [[ "${HAS_TS}" -eq 1 || "${HAS_JS}" -eq 1 ]] && printf "TypeScript/JavaScript "
  [[ "${HAS_PYTHON}" -eq 1 ]] && printf "Python "
  [[ "${HAS_JAVA}" -eq 1 ]] && printf "Java/Kotlin "
)"
info "Changed files: ${#PUSH_FILES[@]}"

# IT 共享前置或依赖变化也会影响场景；所有 Go 改动都编译现有 IT，不执行用例。
run_it_compile_checks() {
  [[ "${HAS_GO}" -eq 1 && -d it ]] || return 0
  command -v go >/dev/null 2>&1 || return 0
  local packages
  packages=$(find it -type f -name '*_test.go' -exec dirname {} \; | sort -u)
  [[ -n "$packages" ]] || return 0
  section "Go: compile component IT"
  CHECKS_RUN=$((CHECKS_RUN + 1))
  local package output
  output=$(mktemp)
  while IFS= read -r package; do
    if ! go test -c -tags=it -o /dev/null "./$package" >"$output" 2>&1; then
      error "IT compilation failed: $package"
      show_output "go test -c -tags=it ./$package" "$output"
      FAILED=1
    fi
  done <<< "$packages"
  rm -f "$output"
}

run_go_checks
run_it_compile_checks
run_rust_checks
run_ts_js_checks
run_python_checks
run_java_checks
check_dependency_changes

# ── Final result ────────────────────────────────────────────────
if [[ "${FAILED}" -ne 0 ]]; then
  echo "" >&2
  echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}" >&2
  error "pre-push checks FAILED (${CHECKS_RUN} checks run). Push blocked."
  echo "" >&2
  echo -e "  ${CYAN}To fix:${RESET}" >&2
  echo -e "    1. Read the error output above (full compiler/linter/test output included)" >&2
  echo -e "    2. Fix each reported issue" >&2
  echo -e "    3. Commit the fixes" >&2
  echo -e "    4. Retry: git push" >&2
  echo "" >&2
  echo -e "  ${CYAN}Emergency override:${RESET} SKIP_PRE_PUSH=1 git push" >&2
  echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}" >&2
  exit 1
fi

echo -e "${GREEN}[OK]${RESET} pre-push: all ${CHECKS_RUN} checks passed (build + lint + test)"
