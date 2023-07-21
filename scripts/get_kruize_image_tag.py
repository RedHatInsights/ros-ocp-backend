import json
import sys
import yaml

def create_commitsha(yaml_file):

    # Read YAML file
    with open(yaml_file, 'r') as f:
        data = yaml.load(f, Loader=yaml.SafeLoader)

    # Write YAML object to JSON format
    with open('/tmp/kruize-clowdapp.json', 'w') as f:
        json.dump(data, f, sort_keys=False)

    # Read JSON file into Python dict
    with open('/tmp/kruize-clowdapp.json', 'r') as f:
        json_data = json.load(f)

    # Fetch the kruize image tag value
    param = json_data["parameters"]
    for data in param:
        if data["name"] == "KRUIZE_IMAGE_TAG":
            commit_tag=data["value"]
            
    with open('/tmp/.commitsha', 'w') as f:
        f.write(commit_tag + "\n")
        f.close()

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python get_kruize_image_tag.py <path to kruize clowdapp yaml file> ")
        sys.exit(1)

    yaml_file = sys.argv[1]
    create_commitsha(yaml_file)
