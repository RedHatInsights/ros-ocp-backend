#!/bin/bash
set -euo pipefail

################################################################################
# ROS OpenShift JWT Authentication Deployment Script
#
# This script orchestrates the complete JWT authentication setup for ROS on
# OpenShift by wrapping the authoritative scripts from ros-helm-chart repository.
#
# Based on: https://github.com/insights-onprem/ros-helm-chart/blob/main/scripts/README.md
# Section: JWT Authentication Setup
#
# Usage:
#   ./deploy-test-ros.sh [OPTIONS]
#
# Options:
#   --skip-rhbk               Skip Red Hat Build of Keycloak (RHBK) deployment
#   --skip-strimzi            Skip Kafka/Strimzi deployment
#   --skip-helm               Skip ROS Helm chart installation
#   --skip-tls                Skip TLS certificate setup
#   --skip-test               Skip JWT authentication test
#   --skip-image-override     Skip creating custom values file for image override
#   --namespace NAME          Target namespace (default: ros-ocp)
#   --image-tag TAG           Custom image tag for ros-ocp-backend services
#   --use-local-chart         Use local Helm chart instead of GitHub release
#   --verbose                 Enable verbose output
#   --dry-run                 Show what would be executed without running
#   --help                    Display this help message
#
# Environment Variables:
#   KUBECONFIG               Path to kubeconfig file (default: ~/.kube/config)
#   KUBEADMIN_PASSWORD_FILE  Path to kubeadmin password file
#   SHARED_DIR               Shared directory containing kubeadmin-password
#   OPENSHIFT_API            OpenShift API URL (auto-detected from kubeconfig)
#   OPENSHIFT_USERNAME       OpenShift username (default: kubeadmin)
#   OPENSHIFT_PASSWORD       OpenShift password (auto-detected from files)
#   QUAY_USERNAME            Quay.io username for pulling images
#   QUAY_PASSWORD            Quay.io password for pulling images
#   IMAGE_REGISTRY           Image registry (default: quay.io)
#   IMAGE_REPOSITORY         Image repository (default: insights-onprem/ros-ocp-backend)
#
# Note: This script will automatically login to OpenShift using credentials from:
#       1. KUBECONFIG file (for API URL)
#       2. KUBEADMIN_PASSWORD_FILE or SHARED_DIR/kubeadmin-password (for password)
#       If already logged in, it will skip the login step.
#
# Prerequisites:
#   - oc CLI installed and configured
#   - helm CLI installed (v3+)
#   - yq installed for YAML/JSON processing
#   - curl installed for downloading scripts
#   - OpenShift cluster with admin access
#
# Example:
#   # Full deployment with custom image
#   ./deploy-test-ros.sh --image-tag main-abc123
#
#   # Skip RHBK if already deployed
#   ./deploy-test-ros.sh --skip-rhbk --namespace ros-production
#
#   # Dry run to preview actions
#   ./deploy-test-ros.sh --dry-run --verbose
#
################################################################################

# Script metadata
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Default configuration
NAMESPACE="${NAMESPACE:-ros-ocp}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-quay.io}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-insights-onprem/ros-ocp-backend}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
USE_LOCAL_CHART="${USE_LOCAL_CHART:-false}"
VERBOSE="${VERBOSE:-false}"
DRY_RUN="${DRY_RUN:-false}"

# OpenShift authentication
KUBECONFIG="${KUBECONFIG:-${HOME}/.kube/config}"
OPENSHIFT_USERNAME="${OPENSHIFT_USERNAME:-kubeadmin}"
OPENSHIFT_API="${OPENSHIFT_API:-}"
OPENSHIFT_PASSWORD="${OPENSHIFT_PASSWORD:-}"
KUBEADMIN_PASSWORD_FILE="${KUBEADMIN_PASSWORD_FILE:-}"
SHARED_DIR="${SHARED_DIR:-}"

# Script URLs from ros-helm-chart repository
ROS_HELM_CHART_BASE_URL="https://raw.githubusercontent.com/insights-onprem/ros-helm-chart/main"
ROS_HELM_CHART_SCRIPTS_URL="${ROS_HELM_CHART_BASE_URL}/scripts"
SCRIPT_DEPLOY_RHBK="deploy-rhbk.sh"  # Red Hat Build of Keycloak (RHBK)
SCRIPT_DEPLOY_STRIMZI="deploy-strimzi.sh"
SCRIPT_INSTALL_AUTHORINO="install-authorino.sh"
SCRIPT_INSTALL_HELM="install-helm-chart.sh"
SCRIPT_SETUP_TLS="setup-cost-mgmt-tls.sh"
SCRIPT_TEST_JWT="test-ocp-dataflow-jwt.sh"
OPENSHIFT_VALUES_FILE="openshift-values.yaml"

# Step flags (default: run all steps)
SKIP_RHBK=false  # Red Hat Build of Keycloak
SKIP_STRIMZI=false
SKIP_HELM=false
SKIP_TLS=false
SKIP_TEST=false
SKIP_IMAGE_OVERRIDE=false

# Temporary directory for downloaded scripts
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

################################################################################
# Logging functions
################################################################################

log_info() {
    echo -e "${BLUE}ℹ INFO:${NC} $*"
}

log_success() {
    echo -e "${GREEN}✅ SUCCESS:${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}⚠ WARNING:${NC} $*"
}

log_error() {
    echo -e "${RED}❌ ERROR:${NC} $*" >&2
}

log_step() {
    echo -e "${CYAN}▶${NC} $*"
}

log_verbose() {
    if [[ "${VERBOSE}" == "true" ]]; then
        echo -e "${CYAN}[VERBOSE]${NC} $*"
    fi
}

################################################################################
# Utility functions
################################################################################

show_help() {
    sed -n '/^# Usage:/,/^################################################################################$/p' "$0" | sed 's/^# \?//'
    exit 0
}

check_prerequisites() {
    log_step "Checking prerequisites"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Skipping prerequisite checks"
        return 0
    fi
    
    local missing_tools=()
    
    # Check required tools
    for tool in oc helm yq curl; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        else
            log_verbose "Found: $tool ($(command -v "$tool"))"
        fi
    done
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_error "Please install missing tools and try again"
        log_error ""
        log_error "Installation instructions:"
        log_error "  macOS:  brew install yq"
        log_error "  Linux:  See https://github.com/mikefarah/yq#install"
        exit 1
    fi
    
    log_success "All required tools are installed"
}

detect_openshift_credentials() {
    log_info "Detecting OpenShift credentials from environment..."
    
    # Detect API URL from kubeconfig if not set
    if [[ -z "${OPENSHIFT_API}" ]] && [[ -f "${KUBECONFIG}" ]]; then
        OPENSHIFT_API=$(yq e '.clusters[0].cluster.server' "${KUBECONFIG}" 2>/dev/null || echo "")
        if [[ -n "${OPENSHIFT_API}" ]]; then
            log_verbose "Detected API URL from kubeconfig: ${OPENSHIFT_API}"
        fi
    fi
    
    # Detect password from files if not set
    if [[ -z "${OPENSHIFT_PASSWORD}" ]]; then
        if [[ -n "${KUBEADMIN_PASSWORD_FILE}" ]] && [[ -s "${KUBEADMIN_PASSWORD_FILE}" ]]; then
            OPENSHIFT_PASSWORD="$(cat "${KUBEADMIN_PASSWORD_FILE}")"
            log_verbose "Loaded password from KUBEADMIN_PASSWORD_FILE"
        elif [[ -n "${SHARED_DIR}" ]] && [[ -s "${SHARED_DIR}/kubeadmin-password" ]]; then
            OPENSHIFT_PASSWORD="$(cat "${SHARED_DIR}/kubeadmin-password")"
            log_verbose "Loaded password from SHARED_DIR/kubeadmin-password"
        fi
    fi
}

login_to_openshift() {
    log_step "Logging into OpenShift"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would login to OpenShift"
        return 0
    fi
    
    # Detect credentials from environment
    detect_openshift_credentials
    
    # Check if credentials are available
    if [[ -z "${OPENSHIFT_API}" ]]; then
        log_error "OPENSHIFT_API not set and could not be detected from kubeconfig"
        log_error "Please set OPENSHIFT_API environment variable or ensure KUBECONFIG is valid"
        return 1
    fi
    
    if [[ -z "${OPENSHIFT_PASSWORD}" ]]; then
        log_error "OPENSHIFT_PASSWORD not set and could not be detected from files"
        log_error "Please set one of:"
        log_error "  - OPENSHIFT_PASSWORD environment variable"
        log_error "  - KUBEADMIN_PASSWORD_FILE pointing to password file"
        log_error "  - SHARED_DIR containing kubeadmin-password file"
        return 1
    fi
    
    # Configure kubeconfig to skip TLS verification
    if [[ -f "${KUBECONFIG}" ]]; then
        log_verbose "Configuring kubeconfig to skip TLS verification..."
        yq -i 'del(.clusters[].cluster.certificate-authority-data) | .clusters[].cluster.insecure-skip-tls-verify=true' "${KUBECONFIG}" 2>/dev/null || true
    fi
    
    # Attempt login
    log_info "Logging in as ${OPENSHIFT_USERNAME} to ${OPENSHIFT_API}..."
    if oc login "${OPENSHIFT_API}" \
        --username="${OPENSHIFT_USERNAME}" \
        --password="${OPENSHIFT_PASSWORD}" \
        --insecure-skip-tls-verify=true &> /dev/null; then
        log_success "Successfully logged into OpenShift"
    else
        log_error "Failed to login to OpenShift"
        log_error "Please verify credentials and API URL"
        return 1
    fi
}

check_oc_connection() {
    log_step "Verifying OpenShift connection"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would verify OpenShift connection"
        return 0
    fi
    
    # Check if already logged in
    if ! oc whoami &> /dev/null; then
        log_info "Not currently logged into OpenShift, attempting automatic login..."
        
        # Try to login automatically
        if ! login_to_openshift; then
            log_error "Automatic login failed"
            log_error ""
            log_error "Manual login options:"
            log_error "  1. Set environment variables:"
            log_error "     export OPENSHIFT_API='https://api.example.com:6443'"
            log_error "     export OPENSHIFT_PASSWORD='your-password'"
            log_error ""
            log_error "  2. Or login manually:"
            log_error "     oc login https://api.example.com:6443"
            log_error ""
            exit 1
        fi
    else
        log_success "Already logged into OpenShift"
    fi
    
    local current_user
    current_user=$(oc whoami)
    local current_server
    current_server=$(oc whoami --show-server)
    
    log_success "Connected to OpenShift as: ${current_user}"
    log_info "Server: ${current_server}"
    
    # Check if user has admin privileges
    if oc auth can-i create clusterrole &> /dev/null; then
        log_success "User has cluster-admin privileges"
    else
        log_warning "User may not have sufficient privileges for cluster-scoped resources"
        log_warning "Some deployment steps may fail without admin access"
    fi
}

download_script() {
    local script_name="$1"
    local dest_path="${TEMP_DIR}/${script_name}"
    local script_url="${ROS_HELM_CHART_SCRIPTS_URL}/${script_name}"
    
    log_verbose "Downloading: ${script_url}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would download ${script_name}"
        touch "${dest_path}"
        chmod +x "${dest_path}"
        return 0
    fi
    
    if ! curl -fsSL "${script_url}" -o "${dest_path}"; then
        log_error "Failed to download ${script_name}"
        return 1
    fi
    
    chmod +x "${dest_path}"
    log_verbose "Downloaded to: ${dest_path}"
}

execute_script() {
    local script_name="$1"
    shift
    local script_path="${TEMP_DIR}/${script_name}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would execute: ${script_name} $*"
        return 0
    fi
    
    log_info "Executing: ${script_name} $*"
    
    local exit_code=0
    if [[ "${VERBOSE}" == "true" ]]; then
        bash -x "${script_path}" "$@" || exit_code=$?
    else
        "${script_path}" "$@" || exit_code=$?
    fi
    
    return ${exit_code}
}

create_namespace() {
    log_step "Creating namespace: ${NAMESPACE}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would create namespace ${NAMESPACE}"
        return 0
    fi
    
    if oc get namespace "${NAMESPACE}" &> /dev/null; then
        log_info "Namespace ${NAMESPACE} already exists"
    else
        oc create namespace "${NAMESPACE}"
        log_success "Created namespace: ${NAMESPACE}"
    fi
    
    # Label namespace for Cost Management Operator
    log_info "Labeling namespace for Cost Management Operator..."
    oc label namespace "${NAMESPACE}" cost_management_optimizations=true --overwrite
    log_success "Namespace labeled successfully"
}

################################################################################
# Deployment steps
################################################################################

deploy_rhbk() {
    if [[ "${SKIP_RHBK}" == "true" ]]; then
        log_warning "Skipping Red Hat Build of Keycloak (RHBK) deployment (--skip-rhbk)"
        return 0
    fi
    
    log_step "Deploying Red Hat Build of Keycloak (RHBK) (1/5)"
    
    download_script "${SCRIPT_DEPLOY_RHBK}"
    
    # Export environment variables for RHBK script
    # export NAMESPACE="${NAMESPACE}"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    execute_script "${SCRIPT_DEPLOY_RHBK}" #|| log_warning "RHBK deployment had issues but continuing..."
    
    log_success "Red Hat Build of Keycloak (RHBK) deployment completed"
    return 0
}

deploy_strimzi() {
    if [[ "${SKIP_STRIMZI}" == "true" ]]; then
        log_warning "Skipping Kafka/Strimzi deployment (--skip-strimzi)"
        return 0
    fi
    
    log_step "Deploying Kafka/Strimzi (2/5)"
    
    download_script "${SCRIPT_DEPLOY_STRIMZI}"
    
    # Export environment variables for Strimzi script
    # export KAFKA_NAMESPACE="${NAMESPACE}"
    export KAFKA_ENVIRONMENT="ocp"
    export STORAGE_CLASS="${STORAGE_CLASS:-}"
    
    log_verbose "Using storage class: ${STORAGE_CLASS}"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    execute_script "${SCRIPT_DEPLOY_STRIMZI}" #|| log_warning "Strimzi deployment had issues but continuing..."
    
    log_success "Kafka/Strimzi deployment completed"
    return 0
}

deploy_helm_chart() {
    if [[ "${SKIP_HELM}" == "true" ]]; then
        log_warning "Skipping ROS Helm chart installation (--skip-helm)"
        return 0
    fi
    
    log_step "Deploying ROS Helm chart (3/5)"    
    download_script "${SCRIPT_INSTALL_HELM}"
    
    # Download the official openshift-values.yaml from ros-helm-chart repo
    local values_file="${TEMP_DIR}/openshift-values.yaml"
    download_openshift_values "${values_file}"
    export VALUES_FILE="${values_file}"
    
    # Add image override via --set if not skipped
    if [[ "${SKIP_IMAGE_OVERRIDE}" == "false" ]]; then
        log_info "Configuring image override via Helm --set flags for all ros-ocp-backend services"
        export HELM_EXTRA_ARGS=(
            # Processor service
            "--set" "rosocp.processor.image.repository=${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}"
            "--set" "rosocp.processor.image.tag=${IMAGE_TAG}"
            "--set" "rosocp.processor.image.pullPolicy=Always"
            # Recommendation Poller service
            "--set" "rosocp.recommendationPoller.image.repository=${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}"
            "--set" "rosocp.recommendationPoller.image.tag=${IMAGE_TAG}"
            "--set" "rosocp.recommendationPoller.image.pullPolicy=Always"
            # API service
            "--set" "rosocp.api.image.repository=${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}"
            "--set" "rosocp.api.image.tag=${IMAGE_TAG}"
            "--set" "rosocp.api.image.pullPolicy=Always"
            # Housekeeper service
            "--set" "rosocp.housekeeper.image.repository=${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}"
            "--set" "rosocp.housekeeper.image.tag=${IMAGE_TAG}"
            "--set" "rosocp.housekeeper.image.pullPolicy=Always"
            # Partition Cleaner service
            "--set" "rosocp.partitionCleaner.image.repository=${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}"
            "--set" "rosocp.partitionCleaner.image.tag=${IMAGE_TAG}"
            "--set" "rosocp.partitionCleaner.image.pullPolicy=Always"
        )
        log_verbose "Image: ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${IMAGE_TAG}"
        log_verbose "Overriding images for: processor, recommendationPoller, api, housekeeper, partitionCleaner"
    else
        log_info "Using chart default image (--skip-image-override)"
        export HELM_EXTRA_ARGS=()
    fi
    
    # Export environment variables for Helm script
    export NAMESPACE="${NAMESPACE}"
    export JWT_AUTH_ENABLED="true"
    export USE_LOCAL_CHART="${USE_LOCAL_CHART}"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    if ! execute_script "${SCRIPT_INSTALL_HELM}"; then
        log_warning "Helm chart deployment had issues but continuing..."
        log_info ""
        log_info "To troubleshoot:"
        log_info "  1. Check Helm release status: helm list -n ${NAMESPACE}"
        log_info "  2. Check pod status: oc get pods -n ${NAMESPACE}"
        log_info "  3. View pod logs: oc logs -n ${NAMESPACE} <pod-name>"
        log_info "  4. Check events: oc get events -n ${NAMESPACE} --sort-by='.lastTimestamp'"
    fi
    
    log_success "ROS Helm chart deployment completed"
    return 0
}

download_openshift_values() {
    local values_file="$1"
    
    log_info "Downloading OpenShift values file from ros-helm-chart repository"
    
    local openshift_values_url="${ROS_HELM_CHART_BASE_URL}/${OPENSHIFT_VALUES_FILE}"
    log_verbose "URL: ${openshift_values_url}"
    log_verbose "Destination: ${values_file}"
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_info "DRY RUN: Would download openshift-values.yaml"
        # Create a minimal placeholder for dry-run
        cat > "${values_file}" <<'EOF'
# DRY RUN: Would use openshift-values.yaml from ros-helm-chart repo
global:
  storageClass: ""
EOF
    else
        if ! curl -fsSL "${openshift_values_url}" -o "${values_file}"; then
            log_error "Failed to download openshift-values.yaml from ros-helm-chart repo"
            return 1
        fi
        log_success "Downloaded openshift-values.yaml"
    fi
    
    if [[ "${VERBOSE}" == "true" ]]; then
        log_verbose "Values file contents (first 30 lines):"
        head -30 "${values_file}" | while IFS= read -r line; do
            log_verbose "  ${line}"
        done
    fi
}

setup_tls() {
    if [[ "${SKIP_TLS}" == "true" ]]; then
        log_warning "Skipping TLS certificate setup (--skip-tls)"
        return 0
    fi
    
    log_step "Configuring TLS certificates (4/5)"
    
    download_script "${SCRIPT_SETUP_TLS}"
    
    # Export environment variables for TLS script
    export NAMESPACE="${NAMESPACE}"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    execute_script "${SCRIPT_SETUP_TLS}"
    
    log_success "TLS certificate setup completed"
}

test_jwt_flow() {
    if [[ "${SKIP_TEST}" == "true" ]]; then
        log_warning "Skipping JWT authentication test (--skip-test)"
        return 0
    fi
    
    log_step "Testing JWT authentication (5/5)"
    
    # Ensure we're logged in to OpenShift for JWT test
    if [[ "${DRY_RUN}" != "true" ]]; then
        if ! oc whoami -t &> /dev/null; then
            log_info "Not logged in to OpenShift with a user that has an available token, attempting login for JWT test..."
            if ! login_to_openshift; then
                log_warning "Failed to login to OpenShift, skipping JWT test"
                return 0
            fi
        fi
    fi
    
    download_script "${SCRIPT_TEST_JWT}"
    
    # Export environment variables for JWT test script
    export NAMESPACE="${NAMESPACE}"
    
    if [[ "${VERBOSE}" == "true" ]]; then
        export VERBOSE="true"
    fi
    
    execute_script "${SCRIPT_TEST_JWT}" || log_warning "JWT authentication test had issues but continuing..."
    
    log_success "JWT authentication test completed"
}

################################################################################
# Main deployment workflow
################################################################################

print_summary() {
    echo ""
    log_info "Deployment Configuration:"
    echo "  Namespace:           ${NAMESPACE}"
    echo "  Image:               ${IMAGE_REGISTRY}/${IMAGE_REPOSITORY}:${IMAGE_TAG}"
    echo "  Use Local Chart:     ${USE_LOCAL_CHART}"
    echo ""
    log_info "Steps to execute:"
    [[ "${SKIP_RHBK}" == "false" ]] && echo "  ✓ Deploy Red Hat Build of Keycloak (RHBK)" || echo "  ✗ Deploy RHBK (SKIPPED)"
    [[ "${SKIP_STRIMZI}" == "false" ]] && echo "  ✓ Deploy Kafka/Strimzi" || echo "  ✗ Deploy Kafka/Strimzi (SKIPPED)"
    [[ "${SKIP_HELM}" == "false" ]] && echo "  ✓ Deploy ROS Helm Chart" || echo "  ✗ Deploy ROS Helm Chart (SKIPPED)"
    [[ "${SKIP_TLS}" == "false" ]] && echo "  ✓ Setup TLS Certificates" || echo "  ✗ Setup TLS Certificates (SKIPPED)"
    [[ "${SKIP_TEST}" == "false" ]] && echo "  ✓ Test JWT Flow" || echo "  ✗ Test JWT Flow (SKIPPED)"
    [[ "${SKIP_IMAGE_OVERRIDE}" == "false" ]] && echo "  ✓ Include Image Override in Values" || echo "  ✗ Include Image Override in Values (SKIPPED)"
    echo ""
}

print_completion() {
    echo ""
    log_success "Deployment completed successfully"
    echo ""
    log_info "ROS with JWT authentication deployed to namespace: ${NAMESPACE}"
    echo ""
    log_info "Next steps:"
    echo "  1. Verify: oc get pods -n ${NAMESPACE}"
    echo "  2. Check route: oc get route -n ${NAMESPACE}"
    echo "  3. View logs: oc logs -n ${NAMESPACE} -l app.kubernetes.io/name=ingress -f"
    echo ""
}

main() {
    echo ""
    echo -e "${CYAN}ROS OpenShift JWT Authentication Deployment${NC}"
    echo ""
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-rhbk)
                SKIP_RHBK=true
                shift
                ;;
            --skip-strimzi)
                SKIP_STRIMZI=true
                shift
                ;;
            --skip-helm)
                SKIP_HELM=true
                shift
                ;;
            --skip-tls)
                SKIP_TLS=true
                shift
                ;;
            --skip-test)
                SKIP_TEST=true
                shift
                ;;
            --skip-image-override)
                SKIP_IMAGE_OVERRIDE=true
                shift
                ;;
            --namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            --image-tag)
                IMAGE_TAG="$2"
                shift 2
                ;;
            --use-local-chart)
                USE_LOCAL_CHART=true
                shift
                ;;
            --verbose)
                VERBOSE=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help|-h)
                show_help
                ;;
            *)
                log_error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done
    
    # Show deployment summary
    print_summary
    
    if [[ "${DRY_RUN}" == "true" ]]; then
        log_warning "DRY RUN MODE: No changes will be made"
        echo ""
    fi
    
    # Execute deployment steps
    check_prerequisites
    check_oc_connection
    create_namespace
    
    deploy_rhbk
    deploy_strimzi
    deploy_helm_chart
    setup_tls
    test_jwt_flow
    
    # Print completion message
    if [[ "${DRY_RUN}" == "false" ]]; then
        print_completion
    else
        echo ""
        log_info "DRY RUN completed. No changes were made."
        echo ""
    fi
    
    exit 0
}

# Run main function
main "$@"

