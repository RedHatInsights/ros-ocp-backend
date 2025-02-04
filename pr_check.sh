#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
export APP_NAME="ros"  # name of app-sre "application" folder this component lives in
export COMPONENT_NAME="kruize ros-ocp-backend"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
export COMPONENTS="kruize ros-ocp-backend"
export IMAGE="quay.io/cloudservices/ros-ocp-backend"
export DOCKERFILE="Dockerfile"

export IQE_PLUGINS="ros_ocp"
export IQE_MARKER_EXPRESSION="smoke"
export IQE_FILTER_EXPRESSION=""
export IQE_CJI_TIMEOUT="30m"
export IQE_ENV_VARS="JOB_NAME=${JOB_NAME},BUILD_NUMBER=${BUILD_NUMBER},BUILD_URL=${BUILD_URL}"
export IQE_PARALLEL_ENABLED="false"

# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s $CICD_URL/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh

source $CICD_ROOT/build.sh

# Deploy to an ephemeral namespace for testing
source $CICD_ROOT/deploy_ephemeral_env.sh


# Creating perf profile
retries=10
for ((i=1; i<=retries; i++)); do
    echo "Starting to create performance profile"

    if [[ $i == "1" ]]; then
        service="kruize-recommendations"
        oc expose svc/${service} -n ${NAMESPACE}
        SERVER_IP=($(oc status --namespace=${NAMESPACE} | grep ${service} | grep port | cut -d " " -f1 | cut -d "/" -f3))
        echo "IP = $SERVER_IP"
        KRUIZE_URL="http://${SERVER_IP}"
    fi

    http_response=$(curl -s -H 'Accept: application/json' -w "%{http_code}" -o /dev/null ${KRUIZE_URL}/createPerformanceProfile -d @./resource_optimization_openshift.json)

    if [[ $http_response == "201" ]]; then
        echo "Performance profile created successfully!"
        break
    elif [[ $http_response == "409" ]]; then
        echo "Performance profile already exists!"
        break
    else
        echo "Failed to create the performance profile! Waiting for 3 seconds, then retry"
        sleep 3
    fi

    if [[ $i == $retries ]]; then
        echo "Failed to create performance profile after 10 retries!"
        exit 1
    fi
done

# Run iqe-ros-ocp smoke tests with ClowdJobInvocation
export COMPONENT_NAME="ros-ocp-backend"
source $CICD_ROOT/cji_smoke_test.sh

# This will add the Ibutsu URL and test run IDs as a git check on PRs.
source $CICD_ROOT/post_test_results.sh

