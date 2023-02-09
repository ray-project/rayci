import copy
import glob
import json
import os
import sys
from pathlib import Path
from typing import Dict, List

import click
import yaml


BASE_STEPS_JSON = Path(__file__).parent / "step.json"


ALLOW_STEP_OVERRIDE = {"label", "commands", "if"}

SPECIAL_FIELD_IMAGE = "image"
SPECIAL_FIELD_INSTANCE_SIZE = "instance_size"

DEFAULT_INSTANCE_SIZE = "large"

INSTANCE_SIZE_TO_QUEUE = {
    "small": os.environ.get("ECOSYSTEM_QUEUE_SMALL", "__ecosystem_queue_small"),
    "medium": os.environ.get("ECOSYSTEM_QUEUE_MEDIUM", "__ecosystem_queue_medium"),
    "large": os.environ.get("ECOSYSTEM_QUEUE_LARGE", "__ecosystem_queue_large"),
    "gpu": os.environ.get("ECOSYSTEM_QUEUE_GPU", "__ecosystem_queue_gpu"),
}


def load_pipeline_steps(base_step: Dict, pipeline_file_path: str) -> List[Dict]:
    steps = []
    with open(pipeline_file_path, "r") as fp:
        jobs = yaml.safe_load(fp)

    for job in jobs:
        steps.append(job_to_step(base_step, job))

    return steps


def job_to_step(base_step: Dict, job: Dict) -> Dict:
    step = copy.deepcopy(base_step)
    job = job.copy()

    image = job.pop(SPECIAL_FIELD_IMAGE, None)
    if image:
        docker_key = list(base_step["plugins"][0].keys())[0]
        step["plugins"][0][docker_key]["image"] = image

    instance_size = job.pop(SPECIAL_FIELD_INSTANCE_SIZE, DEFAULT_INSTANCE_SIZE)
    try:
        step["agents"]["queue"] = INSTANCE_SIZE_TO_QUEUE[instance_size]
    except Exception as e:
        raise ValueError(
            f"Invalid instance size: {instance_size}. Choose from "
            f"{list(INSTANCE_SIZE_TO_QUEUE.keys())}"
        ) from e

    for key, val in job.items():
        if key not in ALLOW_STEP_OVERRIDE:
            raise ValueError(f"Cannot override field: {key}")

        step[key] = val

    return step


@click.command()
@click.argument("pipelines", required=True, type=str)
def main(
    pipelines: str
):
    with open(BASE_STEPS_JSON, "r") as f:
        base_step = json.load(f)

    all_steps = []

    for pipeline in glob.glob(pipelines):
        all_steps += load_pipeline_steps(base_step, pipeline)

    # Print to stdout
    steps_str = json.dumps(all_steps)
    print(steps_str, file=sys.stderr)
    print(steps_str)


if __name__ == "__main__":
    main()
