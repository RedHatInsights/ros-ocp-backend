def get_all_namespaces(df):
    return df.namespace.unique().tolist()


def get_all_deployments_from_namespace(df, namespace):
    return df.loc[df['namespace'] == namespace, 'deployment_name'].unique().tolist()


def get_all_containers_and_images_from_deployment(df, namespace, deployment_name):
    data = df.query('namespace==@namespace & deployment_name==@deployment_name')[["container_name", "image_name"]]
    return data.to_dict('records')


def get_all_containers_and_metrics(df, namespace, deployment_name):
    data = df.query('namespace==@namespace & deployment_name==@deployment_name')
    return data.to_dict('records')


def make_container_data(container):
    container_data = {}
    container_data["image_name"] = container["image_name"]
    container_data["container_name"] = container["container_name"]
    container_data["container_metrics"] = {}

    # cpuRequest
    container_data["container_metrics"]["cpuRequest"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["cpuRequest"]["results"]["general_info"]["sum"] = container["cpu_request_sum_container"]
    container_data["container_metrics"]["cpuRequest"]["results"]["general_info"]["mean"] = container["cpu_request_avg_container"]
    container_data["container_metrics"]["cpuRequest"]["results"]["general_info"]["units"] = "cores"

    # cpuLimit
    container_data["container_metrics"]["cpuLimit"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["cpuLimit"]["results"]["general_info"]["sum"] = container["cpu_limit_sum_container"]
    container_data["container_metrics"]["cpuLimit"]["results"]["general_info"]["mean"] = container["cpu_limit_avg_container"]
    container_data["container_metrics"]["cpuLimit"]["results"]["general_info"]["units"] = "cores"

    # cpuUsage
    container_data["container_metrics"]["cpuUsage"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["cpuUsage"]["results"]["general_info"]["max"] = container["cpu_usage_max_container"]
    container_data["container_metrics"]["cpuUsage"]["results"]["general_info"]["mean"] = container["cpu_usage_avg_container"]
    container_data["container_metrics"]["cpuUsage"]["results"]["general_info"]["units"] = "cores"

    # cpuThrottle
    container_data["container_metrics"]["cpuThrottle"] = {"results": {"general_info": {}}}
    # Below needs to change once "cpu_throttle_max_container" is added in tar archive
    container_data["container_metrics"]["cpuThrottle"]["results"]["general_info"]["max"] = container["cpu_throttle_avg_container"]
    container_data["container_metrics"]["cpuThrottle"]["results"]["general_info"]["mean"] = container["cpu_throttle_avg_container"]
    container_data["container_metrics"]["cpuThrottle"]["results"]["general_info"]["units"] = "cores"

    # memoryRequest
    container_data["container_metrics"]["memoryRequest"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["memoryRequest"]["results"]["general_info"]["sum"] = container["mem_request_sum_container"]
    container_data["container_metrics"]["memoryRequest"]["results"]["general_info"]["mean"] = container["mem_request_avg_container"]
    container_data["container_metrics"]["memoryRequest"]["results"]["general_info"]["units"] = "MiB"

    # memoryLimit
    container_data["container_metrics"]["memoryLimit"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["memoryLimit"]["results"]["general_info"]["sum"] = container["mem_limit_sum_container"]
    container_data["container_metrics"]["memoryLimit"]["results"]["general_info"]["mean"] = container["mem_limit_avg_container"]
    container_data["container_metrics"]["memoryLimit"]["results"]["general_info"]["units"] = "MiB"

    # memoryUsage
    container_data["container_metrics"]["memoryUsage"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["memoryUsage"]["results"]["general_info"]["max"] = container["mem_usage_max_container"]
    container_data["container_metrics"]["memoryUsage"]["results"]["general_info"]["mean"] = container["mem_usage_avg_container"]
    container_data["container_metrics"]["memoryUsage"]["results"]["general_info"]["units"] = "MiB"

    # memoryRSS
    container_data["container_metrics"]["memoryRSS"] = {"results": {"general_info": {}}}
    container_data["container_metrics"]["memoryRSS"]["results"]["general_info"]["max"] = container["mem-rss_usage_max_container"]
    container_data["container_metrics"]["memoryRSS"]["results"]["general_info"]["mean"] = container["mem-rss_usage_avg_container"]
    container_data["container_metrics"]["memoryRSS"]["results"]["general_info"]["units"] = "MiB"

    return container_data
