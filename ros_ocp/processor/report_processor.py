import pandas as pd
from ros_ocp.processor.kruize_api import create_experiments, update_results, list_recommendations


def process_report(consumed_message, request_obj):
    report_files = consumed_message["files"]
    for report in report_files:
        df = pd.read_csv(report)
        create_experiments(df, request_obj)
        list_of_experiments = update_results(df, request_obj)
        for experiment in list_of_experiments:
            list_recommendations(experiment)
