#!/usr/bin/env bash
# pre_commit_check.sh — Universal pre-commit checks (must complete < 5s)
#
# Design principles:
#   - Language-agnostic: auto-detects Go, Rust, TypeScript, Python, etc.
#   - AI-friendly: structured error output with file, line, reason, and fix command
#   - Fast: only file-level static checks, no compilation/lint/tests (those go in pre-push)
#   - Mandatory for all team members (escape hatch: SKIP_PRE_COMMIT=1)
#
# Checks:
#   1. Sensitive file blocking (.env / .pem / .key / credentials etc.)
#   2. Build artifact blocking (vendor/ / node_modules/ / target/ / dist/ etc.)
#   3. Formatting check (gofmt / rustfmt / prettier / black etc.)
#   4. Merge conflict marker detection (<<<<<<<)
#   5. Large file blocking (> 5MB)
#   6. Debug code detection (multi-language)
#
# Optional (via env vars):
#   AI_TEST_GUARD=1  — Prevent AI agents from modifying existing test assertions

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
RESET='\033[0m'

# ── AI-friendly structured output ──────────────────────────────
# Format: [BLOCK] <check_name> | <file>:<line> | <reason> | fix: <command>
error()    { echo -e "${RED}[BLOCK]${RESET} $*" >&2; }
warn()     { echo -e "${YELLOW}[WARN]${RESET} $*" >&2; }
info()     { echo -e "${CYAN}[INFO]${RESET} $*" >&2; }
ai_error() {
  local check="$1" file="$2" line="${3:-0}" reason="$4" fix="${5:-}"
  echo -e "${RED}[BLOCK]${RESET} ${check} | ${file}:${line} | ${reason}" >&2
  if [[ -n "${fix}" ]]; then
    echo -e "  ${CYAN}fix:${RESET} ${fix}" >&2
  fi
}

# ── Global skip ─────────────────────────────────────────────────
if [[ "${SKIP_PRE_COMMIT:-}" == "1" ]]; then
  echo -e "${YELLOW}[SKIP]${RESET} SKIP_PRE_COMMIT=1, skipping all pre-commit checks"
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

# ── Collect staged files ────────────────────────────────────────
STAGED_FILES=()
while IFS= read -r file; do
  [[ -n "$file" ]] && STAGED_FILES+=("$file")
done < <(git diff --cached --name-only --diff-filter=ACMR)

if [[ ${#STAGED_FILES[@]} -eq 0 ]]; then
  echo -e "${GREEN}[OK]${RESET} No staged changes, skipping"
  exit 0
fi

FAILED=0
CHECKS_RUN=0
ERRORS_DETAIL=()

# ── Language detection (from staged files) ──────────────────────
HAS_GO=0; HAS_RUST=0; HAS_TS=0; HAS_JS=0; HAS_PYTHON=0; HAS_JAVA=0; HAS_CPP=0
for f in "${STAGED_FILES[@]}"; do
  case "$f" in
    *.go)                         HAS_GO=1 ;;
    *.rs)                         HAS_RUST=1 ;;
    *.ts|*.tsx)                   HAS_TS=1 ;;
    *.js|*.jsx|*.mjs|*.cjs)      HAS_JS=1 ;;
    *.py|*.pyi)                   HAS_PYTHON=1 ;;
    *.java|*.kt|*.kts)           HAS_JAVA=1 ;;
    *.c|*.cpp|*.cc|*.h|*.hpp)    HAS_CPP=1 ;;
  esac
done

# ══════════════════════════════════════════════════════════════════
# Check 1: Sensitive files
# ══════════════════════════════════════════════════════════════════
check_sensitive_files() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local patterns=(
    '\.env$'
    '\.env\.'
    'credentials'
    '\.pem$'
    '\.key$'
    '\.p12$'
    '\.pfx$'
    '\.jks$'
    '\.keystore$'
    'id_rsa'
    'id_ed25519'
    '\.secret'
    'secrets\.ya?ml$'
    '\.cert$'
    '\.crt$'
    'private.*key'
    'token\.json$'
    'service.account\.json$'
    '\.npmrc$'
    '\.pypirc$'
    '\.docker/config\.json$'
    'kubeconfig'
  )

  local violations=()
  for f in "${STAGED_FILES[@]}"; do
    # Exclude documentation/test/spec directories (false positives)
    [[ "$f" =~ ^(docs/|specs/|README|CHANGELOG|test/fixtures/|tests/fixtures/|__fixtures__/) ]] && continue
    for p in "${patterns[@]}"; do
      if echo "$f" | grep -iE "$p" >/dev/null 2>&1; then
        violations+=("$f")
        break
      fi
    done
  done

  if [[ ${#violations[@]} -gt 0 ]]; then
    error "Sensitive files must not be committed:"
    for v in "${violations[@]}"; do
      ai_error "sensitive_file" "$v" "0" "File matches sensitive pattern — may contain secrets" "git reset HEAD ${v} && echo '${v}' >> .gitignore"
    done
    echo "" >&2
    echo -e "  ${CYAN}Reason:${RESET} These file patterns often contain API keys, passwords, or certificates." >&2
    echo -e "  ${CYAN}Action:${RESET} Add them to .gitignore, or use git-secret / SOPS for encrypted secrets." >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Check 2: Build artifacts
# ══════════════════════════════════════════════════════════════════
check_artifacts() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local patterns=(
    # Universal
    '^vendor/'
    '^node_modules/'
    '^dist/'
    '^build/'
    '^out/'
    # Go
    'cover.*\.out$'
    # Rust
    '^target/'
    # Python
    '^__pycache__/'
    '/__pycache__/'
    '\.pyc$'
    '\.pyo$'
    '^\.eggs/'
    '\.egg-info/'
    # Java
    '^\.gradle/'
    # .NET
    '^bin/Debug/'
    '^bin/Release/'
    '^obj/'
    # Binaries
    '\.exe$'
    '\.dll$'
    '\.so$'
    '\.dylib$'
    '\.a$'
    '\.lib$'
    '\.wasm$'
  )

  local violations=()
  for f in "${STAGED_FILES[@]}"; do
    for p in "${patterns[@]}"; do
      if echo "$f" | grep -E "$p" >/dev/null 2>&1; then
        violations+=("$f")
        break
      fi
    done
  done

  if [[ ${#violations[@]} -gt 0 ]]; then
    error "Build artifacts/generated files must not be committed (should be in .gitignore):"
    for v in "${violations[@]}"; do
      ai_error "build_artifact" "$v" "0" "File is a build artifact or dependency directory" "git reset HEAD ${v} && echo '${v}' >> .gitignore"
    done
    echo "" >&2
    echo -e "  ${CYAN}Action:${RESET} Run: git rm --cached <file> && echo '<pattern>' >> .gitignore" >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Check 3: Formatting (multi-language)
# ══════════════════════════════════════════════════════════════════
check_formatting() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local unformatted=()
  local fix_cmds=()

  # --- Go: gofmt ---
  if [[ "${HAS_GO}" -eq 1 ]] && command -v gofmt >/dev/null 2>&1; then
    local go_files=()
    for f in "${STAGED_FILES[@]}"; do
      [[ "$f" =~ \.go$ ]] || continue
      [[ "$f" =~ _mock\.go$ ]] && continue
      [[ "$f" =~ \.gen\.go$ ]] && continue
      [[ "$f" =~ /mock/ ]] && continue
      [[ "$f" =~ \.pb\.go$ ]] && continue
      [[ -f "$f" ]] && go_files+=("$f")
    done
    for f in "${go_files[@]}"; do
      if [[ -n "$(gofmt -l "$f" 2>/dev/null)" ]]; then
        unformatted+=("$f")
        fix_cmds+=("gofmt -w $f")
      fi
    done
  fi

  # --- Rust: rustfmt ---
  if [[ "${HAS_RUST}" -eq 1 ]] && command -v rustfmt >/dev/null 2>&1; then
    local rust_files=()
    for f in "${STAGED_FILES[@]}"; do
      [[ "$f" =~ \.rs$ ]] || continue
      [[ -f "$f" ]] && rust_files+=("$f")
    done
    for f in "${rust_files[@]}"; do
      if ! rustfmt --check "$f" >/dev/null 2>&1; then
        unformatted+=("$f")
        fix_cmds+=("rustfmt $f")
      fi
    done
  fi

  # --- TypeScript/JavaScript: prettier or deno fmt ---
  if [[ "${HAS_TS}" -eq 1 || "${HAS_JS}" -eq 1 ]]; then
    local ts_js_files=()
    for f in "${STAGED_FILES[@]}"; do
      [[ "$f" =~ \.(ts|tsx|js|jsx|mjs|cjs)$ ]] || continue
      [[ "$f" =~ \.min\. ]] && continue
      [[ "$f" =~ /vendor/ ]] && continue
      [[ -f "$f" ]] && ts_js_files+=("$f")
    done
    if [[ ${#ts_js_files[@]} -gt 0 ]]; then
      if command -v prettier >/dev/null 2>&1; then
        for f in "${ts_js_files[@]}"; do
          if ! prettier --check "$f" >/dev/null 2>&1; then
            unformatted+=("$f")
            fix_cmds+=("prettier --write $f")
          fi
        done
      elif command -v deno >/dev/null 2>&1 && [[ -f "deno.json" || -f "deno.jsonc" ]]; then
        for f in "${ts_js_files[@]}"; do
          if ! deno fmt --check "$f" >/dev/null 2>&1; then
            unformatted+=("$f")
            fix_cmds+=("deno fmt $f")
          fi
        done
      fi
    fi
  fi

  # --- Python: black or ruff format ---
  if [[ "${HAS_PYTHON}" -eq 1 ]]; then
    local py_files=()
    for f in "${STAGED_FILES[@]}"; do
      [[ "$f" =~ \.py$ ]] || continue
      [[ -f "$f" ]] && py_files+=("$f")
    done
    if [[ ${#py_files[@]} -gt 0 ]]; then
      if command -v ruff >/dev/null 2>&1; then
        for f in "${py_files[@]}"; do
          if ! ruff format --check "$f" >/dev/null 2>&1; then
            unformatted+=("$f")
            fix_cmds+=("ruff format $f")
          fi
        done
      elif command -v black >/dev/null 2>&1; then
        for f in "${py_files[@]}"; do
          if ! black --check --quiet "$f" >/dev/null 2>&1; then
            unformatted+=("$f")
            fix_cmds+=("black $f")
          fi
        done
      fi
    fi
  fi

  if [[ ${#unformatted[@]} -gt 0 ]]; then
    error "The following files are not properly formatted:"
    for i in "${!unformatted[@]}"; do
      ai_error "formatting" "${unformatted[$i]}" "0" "File not formatted according to project standards" "${fix_cmds[$i]}"
    done
    echo "" >&2
    echo -e "  ${CYAN}Batch fix:${RESET}" >&2
    # Deduplicate fix commands by tool
    printf "    %s\n" "${fix_cmds[@]}" >&2
    echo "" >&2
    echo -e "  ${CYAN}Then:${RESET} git add ${unformatted[*]}" >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Check 4: Merge conflict markers
# ══════════════════════════════════════════════════════════════════
check_conflict_markers() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local violations=()
  for f in "${STAGED_FILES[@]}"; do
    [[ -f "$f" ]] || continue
    # Only check text files (skip binaries)
    if file --brief --mime "$f" 2>/dev/null | grep -q "text/"; then
      local conflict_lines
      conflict_lines=$(grep -nE '^(<{7}|>{7}|={7})( |$)' "$f" 2>/dev/null || true)
      if [[ -n "$conflict_lines" ]]; then
        violations+=("$f")
        # Show details for AI
        echo -e "  ${RED}Conflict markers in ${f}:${RESET}" >&2
        echo "$conflict_lines" | head -5 | while IFS= read -r line; do
          local linenum="${line%%:*}"
          ai_error "conflict_marker" "$f" "$linenum" "Unresolved merge conflict marker" "Edit ${f}:${linenum} to resolve the conflict, then git add ${f}"
        done
      fi
    fi
  done

  if [[ ${#violations[@]} -gt 0 ]]; then
    echo "" >&2
    error "Unresolved merge conflict markers detected in ${#violations[@]} file(s)"
    echo -e "  ${CYAN}Action:${RESET} Resolve all <<<<<<< / ======= / >>>>>>> markers, then re-stage." >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Check 5: Large files (> 5MB)
# ══════════════════════════════════════════════════════════════════
check_large_files() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local max_size=$((5 * 1024 * 1024))  # 5MB
  local violations=()

  for f in "${STAGED_FILES[@]}"; do
    [[ -f "$f" ]] || continue
    local size
    size=$(wc -c < "$f" 2>/dev/null | tr -d ' ')
    if [[ "${size:-0}" -gt "${max_size}" ]]; then
      local size_mb
      size_mb=$(awk "BEGIN {printf \"%.1f\", ${size} / 1048576}")
      violations+=("$f")
      ai_error "large_file" "$f" "0" "File size ${size_mb}MB exceeds 5MB limit" "git reset HEAD ${f} — consider using Git LFS: git lfs track '${f}'"
    fi
  done

  if [[ ${#violations[@]} -gt 0 ]]; then
    echo "" >&2
    error "Files exceeding 5MB limit cannot be committed:"
    echo -e "  ${CYAN}Action:${RESET} Use Git LFS for large files: git lfs install && git lfs track '<pattern>'" >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Check 6: Debug code detection (multi-language)
# ══════════════════════════════════════════════════════════════════
check_debug_code() {
  CHECKS_RUN=$((CHECKS_RUN + 1))

  local violations=()

  for f in "${STAGED_FILES[@]}"; do
    [[ -f "$f" ]] || continue

    # Only check newly added lines in staged diff
    local added_lines
    added_lines=$(git diff --cached -U0 -- "$f" 2>/dev/null | awk '/^@@ / { in_hunk=1; next } in_hunk && /^\+/ { print substr($0,2) }' || true)
    [[ -z "$added_lines" ]] && continue

    local debug_pattern=""
    local lang_label=""

    # Go: fmt.Print/Println/Printf (exclude test files)
    if [[ "$f" =~ \.go$ ]] && [[ ! "$f" =~ _test\.go$ ]]; then
      debug_pattern='fmt\.Print(ln|f)?\('
      lang_label="Go"
    fi

    # Rust: println!, dbg!, eprintln! (exclude test files)
    if [[ "$f" =~ \.rs$ ]] && [[ ! "$f" =~ /tests/ ]] && [[ ! "$f" =~ _test\.rs$ ]]; then
      debug_pattern='(println!|dbg!|eprintln!)\('
      lang_label="Rust"
    fi

    # TypeScript/JavaScript: console.log/warn/debug (exclude test files)
    if [[ "$f" =~ \.(ts|tsx|js|jsx|mjs|cjs)$ ]] && [[ ! "$f" =~ \.(test|spec)\. ]]; then
      debug_pattern='console\.(log|warn|debug|info)\('
      lang_label="TypeScript/JavaScript"
    fi

    # Python：print 可用于 CLI 正常输出；阻断断点和显式调试器调用（排除测试文件）。
    if [[ "$f" =~ \.py$ ]] && [[ ! "$f" =~ (test_|_test\.py$|/tests/) ]]; then
      debug_pattern='(\bbreakpoint[[:space:]]*\(|\b(pdb|ipdb)\.set_trace[[:space:]]*\()'
      lang_label="Python"
    fi

    # Java/Kotlin: System.out.println, e.printStackTrace (exclude test files)
    if [[ "$f" =~ \.(java|kt|kts)$ ]] && [[ ! "$f" =~ (Test\.java$|Test\.kt$|/test/) ]]; then
      debug_pattern='(System\.out\.print|\.printStackTrace\(\))'
      lang_label="Java/Kotlin"
    fi

    # Check language-specific debug patterns
    if [[ -n "$debug_pattern" ]]; then
      local matches
      matches=$(echo "$added_lines" | grep -E "$debug_pattern" | grep -Ev '^[[:space:]]*(//|#)' || true)
      if [[ -n "$matches" ]]; then
        violations+=("$f (${lang_label} debug statement)")
        ai_error "debug_code" "$f" "0" "${lang_label} debug statement found in new code" "Remove debug statements (${debug_pattern}) from ${f}"
        echo "    Added lines matching:" >&2
        echo "$matches" | head -3 | sed 's/^+/    /' >&2
      fi
    fi

    # 临时标记检查；模式与提示文本避免在首次暂存 Hook 自身时命中。
    if file --brief --mime "$f" 2>/dev/null | grep -q "text/"; then
      local hack_lines
      local marker_pattern='TODO[[:space:]]*(REMOVE|DELETE|HACK)|FIXME[[:space:]]*(REMOVE|DELETE)|DO NOT (COMMIT|MERGE)|TEMP '"HACK"
      hack_lines=$(echo "$added_lines" | grep -iE "$marker_pattern" || true)
      if [[ -n "$hack_lines" ]]; then
        violations+=("$f (contains a temporary removal marker)")
        ai_error "temp_marker" "$f" "0" "Temporary marker found — code flagged as not ready to commit" "Remove the temporary marker from ${f}"
        echo "    Matching lines:" >&2
        echo "$hack_lines" | head -3 | sed 's/^+/    /' >&2
      fi
    fi
  done

  if [[ ${#violations[@]} -gt 0 ]]; then
    echo "" >&2
    error "Debug code or temporary markers detected in ${#violations[@]} location(s):"
    echo -e "  ${CYAN}Note:${RESET} Regular TODO/FIXME comments are fine; explicit temporary-removal markers are blocked." >&2
    echo -e "  ${CYAN}Action:${RESET} Remove debug statements, then git add && git commit" >&2
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Optional: AI Agent test modification guard (AI_TEST_GUARD=1)
# ══════════════════════════════════════════════════════════════════
check_ai_test_guard() {
  # Only run when explicitly enabled AND in AI environment
  [[ "${AI_TEST_GUARD:-}" == "1" ]] || return 0

  local is_ai=0
  if [[ "${CLAUDE_CODE:-}" == "1" ]] || [[ "${AI_AGENT:-}" == "1" ]] || \
     [[ "${CODEBUDDY:-}" == "1" ]] || [[ "${CURSOR_AI:-}" == "1" ]] || \
     [[ -n "${CLAUDE_SESSION_ID:-}" ]] || [[ "${CODEX:-}" == "1" ]] || \
     [[ "${GEMINI_CLI:-}" == "1" ]] || [[ "${COPILOT_CLI:-}" == "1" ]]; then
    is_ai=1
  fi
  [[ "${is_ai}" -eq 1 ]] || return 0

  CHECKS_RUN=$((CHECKS_RUN + 1))

  # Multi-language test file patterns
  local test_patterns=(
    '_test\.go$'           # Go
    '\.test\.(ts|tsx|js|jsx)$'  # JS/TS
    '\.spec\.(ts|tsx|js|jsx)$'  # JS/TS
    'test_.*\.py$'         # Python
    '.*_test\.py$'         # Python
    'Test.*\.java$'        # Java
    '.*_test\.rs$'         # Rust
    '/tests/.*\.(go|rs|py|ts|js)$'  # test directories
  )

  # Multi-language assertion patterns
  local assertion_patterns=(
    # Go
    'assert\.|require\.|gomock\.|\.EXPECT\(\)|\.Return\('
    # JS/TS
    'expect\(|\.toBe\(|\.toEqual\(|\.toHaveBeenCalled|\.toThrow\(|\.resolves\.|\.rejects\.'
    # Python
    'assert |self\.assert|pytest\.raises|assertEqual|assertRaises'
    # Rust
    'assert!\(|assert_eq!\(|assert_ne!\('
    # Java
    'assertEquals\(|assertThat\(|verify\(|@Test'
  )

  local assertion_re
  assertion_re=$(IFS='|'; echo "${assertion_patterns[*]}")

  local violations=()
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue

    local is_test=0
    for tp in "${test_patterns[@]}"; do
      if echo "$f" | grep -E "$tp" >/dev/null 2>&1; then
        is_test=1
        break
      fi
    done
    [[ "${is_test}" -eq 1 ]] || continue

    if git diff --cached -U0 -- "$f" | grep -E "^[+-]" | grep -v "^[+-]{3}" | grep -E "${assertion_re}" >/dev/null 2>&1; then
      violations+=("$f")
    fi
  done < <(git diff --cached --name-only --diff-filter=M)

  if [[ ${#violations[@]} -gt 0 ]]; then
    error "[AI_TEST_GUARD] AI agents must not modify existing test assertions:"
    for v in "${violations[@]}"; do
      ai_error "ai_test_guard" "$v" "0" "AI agent modified test assertions in existing test file" "Revert changes to test assertions in ${v} — only humans should modify test expectations"
    done
    echo "" >&2
    echo -e "  ${CYAN}Reason:${RESET} Modifying test assertions to make tests pass can hide real bugs." >&2
    echo -e "  ${CYAN}Override:${RESET} Set SKIP_TEST_GUARD=1 if this is intentional." >&2
    [[ "${SKIP_TEST_GUARD:-}" == "1" ]] && return 0
    return 1
  fi
}

# ══════════════════════════════════════════════════════════════════
# Run all checks
# ══════════════════════════════════════════════════════════════════

check_sensitive_files  || FAILED=1
check_artifacts        || FAILED=1
check_formatting       || FAILED=1
check_conflict_markers || FAILED=1
check_large_files      || FAILED=1
check_debug_code       || FAILED=1
check_ai_test_guard    || FAILED=1

if [[ "${FAILED}" -ne 0 ]]; then
  echo "" >&2
  echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}" >&2
  error "pre-commit checks FAILED (${CHECKS_RUN} checks run). Fix the issues above, then:"
  echo -e "  ${CYAN}1.${RESET} Apply the suggested fixes" >&2
  echo -e "  ${CYAN}2.${RESET} git add <fixed files>" >&2
  echo -e "  ${CYAN}3.${RESET} git commit" >&2
  echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}" >&2
  exit 1
fi

echo -e "${GREEN}[OK]${RESET} pre-commit: all ${CHECKS_RUN} checks passed"
