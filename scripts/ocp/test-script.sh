#!/bin/bash
################################################################################
# Test Script for deploy-ros-jwt.sh
#
# This script validates the deployment script without actually deploying
# anything. It tests:
# - Script syntax
# - Command-line argument parsing
# - Help output
# - Dry-run mode
# - Environment variable handling
#
# Usage:
#   ./test-script.sh
#
################################################################################

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_SCRIPT="${SCRIPT_DIR}/deploy-ros-jwt.sh"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

################################################################################
# Test functions
################################################################################

log_test() {
    echo -e "${BLUE}TEST:${NC} $*"
    TESTS_RUN=$((TESTS_RUN + 1))
}

log_pass() {
    echo -e "${GREEN}✅ PASS:${NC} $*"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

log_fail() {
    echo -e "${RED}❌ FAIL:${NC} $*"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

log_info() {
    echo -e "${BLUE}ℹ INFO:${NC} $*"
}

################################################################################
# Tests
################################################################################

test_script_exists() {
    log_test "Script exists and is readable"
    
    if [[ -f "${DEPLOY_SCRIPT}" ]]; then
        log_pass "Script exists at: ${DEPLOY_SCRIPT}"
    else
        log_fail "Script not found at: ${DEPLOY_SCRIPT}"
        return 1
    fi
}

test_script_executable() {
    log_test "Script is executable"
    
    if [[ -x "${DEPLOY_SCRIPT}" ]]; then
        log_pass "Script is executable"
    else
        log_fail "Script is not executable. Run: chmod +x ${DEPLOY_SCRIPT}"
        return 1
    fi
}

test_script_syntax() {
    log_test "Script has valid bash syntax"
    
    if bash -n "${DEPLOY_SCRIPT}" 2>/dev/null; then
        log_pass "Script syntax is valid"
    else
        log_fail "Script has syntax errors"
        bash -n "${DEPLOY_SCRIPT}"
        return 1
    fi
}

test_help_output() {
    log_test "Help output works"
    
    if "${DEPLOY_SCRIPT}" --help >/dev/null 2>&1; then
        log_pass "Help output works"
    else
        log_fail "Help output failed"
        return 1
    fi
}

test_help_content() {
    log_test "Help output contains usage information"
    
    local help_output
    help_output=$("${DEPLOY_SCRIPT}" --help 2>&1)
    
    if echo "${help_output}" | grep -q "Usage:"; then
        log_pass "Help contains usage information"
    else
        log_fail "Help missing usage information"
        return 1
    fi
}

test_dry_run_mode() {
    log_test "Dry-run mode works"
    
    # Set minimal environment to avoid actual connections
    export DRY_RUN=true
    export VERBOSE=false
    
    local output
    output=$("${DEPLOY_SCRIPT}" --dry-run --skip-test 2>&1 || true)
    
    if echo "${output}" | grep -q "DRY RUN"; then
        log_pass "Dry-run mode works"
    else
        log_fail "Dry-run mode not working"
        return 1
    fi
    
    unset DRY_RUN VERBOSE
}

test_skip_flags() {
    log_test "Skip flags are recognized"
    
    local flags=(
        "skip-rhsso"
        "skip-strimzi"
        "skip-authorino"
        "skip-helm"
        "skip-tls"
        "skip-test"
    )
    
    for flag in "${flags[@]}"; do
        if "${DEPLOY_SCRIPT}" --help 2>&1 | grep -q -- "--${flag}"; then
            log_info "  ✓ --${flag} documented"
        else
            log_fail "  ✗ --${flag} not documented in help"
            return 1
        fi
    done
    
    log_pass "All skip flags are documented"
}

test_namespace_option() {
    log_test "Namespace option works"
    
    export DRY_RUN=true
    
    local output
    output=$("${DEPLOY_SCRIPT}" --namespace test-namespace --dry-run 2>&1 || true)
    
    if echo "${output}" | grep -q "test-namespace"; then
        log_pass "Namespace option works"
    else
        log_fail "Namespace option not working"
        return 1
    fi
    
    unset DRY_RUN
}

test_image_tag_option() {
    log_test "Image tag option works"
    
    export DRY_RUN=true
    
    local output
    output=$("${DEPLOY_SCRIPT}" --image-tag test-tag --dry-run 2>&1 || true)
    
    if echo "${output}" | grep -q "test-tag"; then
        log_pass "Image tag option works"
    else
        log_fail "Image tag option not working"
        return 1
    fi
    
    unset DRY_RUN
}

test_verbose_option() {
    log_test "Verbose option works"
    
    export DRY_RUN=true
    
    local output
    output=$("${DEPLOY_SCRIPT}" --verbose --dry-run 2>&1 || true)
    
    if echo "${output}" | grep -q "VERBOSE"; then
        log_pass "Verbose option works"
    else
        log_fail "Verbose option not working"
        return 1
    fi
    
    unset DRY_RUN
}

test_invalid_option() {
    log_test "Invalid options are rejected"
    
    local output
    output=$("${DEPLOY_SCRIPT}" --invalid-option 2>&1 || true)
    
    if echo "${output}" | grep -q "Unknown option"; then
        log_pass "Invalid options are rejected"
    else
        log_fail "Invalid options not properly handled"
        return 1
    fi
}

test_environment_variables() {
    log_test "Environment variables are respected"
    
    export NAMESPACE="env-test-namespace"
    export IMAGE_TAG="env-test-tag"
    export DRY_RUN=true
    
    local output
    output=$("${DEPLOY_SCRIPT}" --dry-run 2>&1)
    
    if echo "${output}" | grep -q "env-test-namespace" && echo "${output}" | grep -q "env-test-tag"; then
        log_pass "Environment variables are respected"
    else
        log_fail "Environment variables not working"
        return 1
    fi
    
    unset NAMESPACE IMAGE_TAG DRY_RUN
}

test_script_metadata() {
    log_test "Script contains proper metadata"
    
    if grep -q "SCRIPT_DIR=" "${DEPLOY_SCRIPT}"; then
        log_pass "Script has metadata"
    else
        log_fail "Script missing metadata"
        return 1
    fi
}

test_error_handling() {
    log_test "Script has error handling (set -euo pipefail)"
    
    if head -20 "${DEPLOY_SCRIPT}" | grep -q "set -euo pipefail"; then
        log_pass "Script has proper error handling"
    else
        log_fail "Script missing error handling"
        return 1
    fi
}

test_cleanup_trap() {
    log_test "Script has cleanup trap"
    
    if grep -q "trap.*EXIT" "${DEPLOY_SCRIPT}"; then
        log_pass "Script has cleanup trap"
    else
        log_fail "Script missing cleanup trap"
        return 1
    fi
}

test_prerequisite_check() {
    log_test "Script checks prerequisites"
    
    if grep -q "check_prerequisites" "${DEPLOY_SCRIPT}"; then
        log_pass "Script has prerequisite check"
    else
        log_fail "Script missing prerequisite check"
        return 1
    fi
}

test_color_output() {
    log_test "Script has color-coded output"
    
    if grep -q "RED=" "${DEPLOY_SCRIPT}" && grep -q "GREEN=" "${DEPLOY_SCRIPT}"; then
        log_pass "Script has color-coded output"
    else
        log_fail "Script missing color-coded output"
        return 1
    fi
}

################################################################################
# Main test execution
################################################################################

main() {
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════╗"
    echo "║  Testing deploy-ros-jwt.sh                                         ║"
    echo "╚════════════════════════════════════════════════════════════════════╝"
    echo ""
    
    # Run all tests
    test_script_exists || true
    test_script_executable || true
    test_script_syntax || true
    test_help_output || true
    test_help_content || true
    test_dry_run_mode || true
    test_skip_flags || true
    test_namespace_option || true
    test_image_tag_option || true
    test_verbose_option || true
    test_invalid_option || true
    test_environment_variables || true
    test_script_metadata || true
    test_error_handling || true
    test_cleanup_trap || true
    test_prerequisite_check || true
    test_color_output || true
    
    # Print summary
    echo ""
    echo "╔════════════════════════════════════════════════════════════════════╗"
    echo "║  Test Summary                                                      ║"
    echo "╚════════════════════════════════════════════════════════════════════╝"
    echo ""
    echo "  Tests Run:    ${TESTS_RUN}"
    echo "  Tests Passed: ${TESTS_PASSED}"
    echo "  Tests Failed: ${TESTS_FAILED}"
    echo ""
    
    if [[ ${TESTS_FAILED} -eq 0 ]]; then
        echo -e "${GREEN}✅ All tests passed!${NC}"
        echo ""
        return 0
    else
        echo -e "${RED}❌ Some tests failed!${NC}"
        echo ""
        return 1
    fi
}

# Run tests
main "$@"

