import requests
from ros_ocp.processor.utils import (
    get_all_namespaces, get_all_deployments_from_namespace,
    get_all_containers_and_images_from_deployment, get_all_containers_and_metrics,
    make_container_data)
from ros_ocp.lib.config import KRUIZE_URL


def create_experiments(df, request_obj):
    payload_data = {
        "performanceProfile": "resource_optimization", "mode": "monitor", "targetCluster": "remote",
        "trial_settings": {"measurement_duration": "15min"}, "recommendation_settings": {"threshold": "0.1"}
        }
    namspaces = get_all_namespaces(df)
    for namespace in namspaces:
        deployments = get_all_deployments_from_namespace(df, namespace)
        for deployment in deployments:
            containers = get_all_containers_and_images_from_deployment(df, namespace, deployment)
            payload_data["experiment_name"] = request_obj["org_id"] + request_obj["cluster_id"] + namespace + deployment
            payload_data['namespace'] = namespace
            payload_data['deployment_name'] = deployment
            payload_data['containers'] = containers
            call_create_experiment(payload_data)


def update_results(df, request_obj):
    list_of_experiments = []
    namspaces = get_all_namespaces(df)
    for namespace in namspaces:
        deployments = get_all_deployments_from_namespace(df, namespace)
        for deployment in deployments:
            containers_with_metrics = get_all_containers_and_metrics(df, namespace, deployment)
            payload_data = {}
            payload_data["experiment_name"] = request_obj["org_id"] + request_obj["cluster_id"] + namespace + deployment

            # Below trial info needs to changed.
            payload_data["info"] = {"trial_info": {"trial_number": 98, "trial_timestamp": "yyyymmddhhmmss"}}

            deployment_data = {}
            deployment_data["deployment_name"] = deployment
            deployment_data["namespace"] = namespace
            deployment_data["pod_metrics"] = []
            all_containers = []
            for container in containers_with_metrics:
                container_data = make_container_data(container)
                all_containers.append(container_data)

            deployment_data["containers"] = all_containers
            payload_data["deployments"] = [deployment_data]
            list_of_experiments.append({"experiment_name": payload_data["experiment_name"],
                                        "deployment_name": deployment,
                                        "namespace": namespace
                                        })
            call_update_result(payload_data)
    return list_of_experiments


def list_recommendations(experiment):
    params = {'experiment_name': experiment["experiment_name"],
              'deployment_name': experiment["deployment_name"],
              'namespace': experiment["namespace"]}
    call_list_recommendations(params)


def call_create_experiment(data):
    print("\n*******************create call*******************************")
    url = KRUIZE_URL + "/createExperiment"
    response = requests.post(url, json=[data])
    print("Response status code = ", response.status_code)
    print(response.text)


def call_update_result(data):
    print("\n*******************update call*******************************")
    url = KRUIZE_URL + "/updateResults"
    response = requests.post(url, json=[data])
    print("Response status code = ", response.status_code)
    print(response.text)


def call_list_recommendations(data):
    print("\n************************************************************")
    url = KRUIZE_URL + "/listRecommendations"
    response = requests.get(url, params=data)
    print("Response status code = ", response.status_code)
    print(response.text)
