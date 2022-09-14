import collections
import json
import os
from pathlib import Path
from typing import Any, Dict, List, Optional

import click
import yaml


def read_pipeline(pipeline_path: Path):
    with open(pipeline_path, "r") as f:
        steps = yaml.safe_load(f)
    return steps


def filter_pipeline_conditions(
    steps: List[Dict[str, Any]],
    key: str = "conditions",
    include: Optional[List[str]] = None,
    exclude: Optional[List[str]] = None,
):
    new_steps = []
    for step in steps:
        conditions = step.get(key, [])
        if include:
            if not any(cond in conditions for cond in include):
                continue
        if exclude:
            if any(cond in conditions for cond in exclude):
                continue
        new_steps.append(step)
    return new_steps


def deep_update(d, u) -> Dict:
    for k, v in u.items():
        if isinstance(v, collections.abc.Mapping):
            d[k] = deep_update(d.get(k, {}), v)
        else:
            d[k] = v
    return d


def update_steps(steps: List[Dict[str, Any]], update: Dict[str, Any]):
    steps = steps.copy()
    for step in steps:
        deep_update(step, update)
    return steps


def drop_pipeline_keys(steps: List[Dict[str, Any]], keys: List[str]):
    steps = steps.copy()
    for step in steps:
        for key in keys:
            step.pop(key, None)
    return steps


@click.command()
@click.argument("pipeline", required=True, type=str)
@click.option("--image", type=str, default=None)
@click.option("--queue", type=str, default=None)
@click.option("--early-only", is_flag=True, default=False)
@click.option("--not-early-only", is_flag=True, default=False)
def main(
    pipeline: str,
    image: Optional[str] = None,
    queue: Optional[str] = None,
    early_only: bool = False,
    not_early_only: bool = False,
):
    if not image:
        raise ValueError("Please specify a docker image using --image")

    if not image:
        raise ValueError("Please specify a runner queue using --queue")

    if early_only and not_early_only:
        raise ValueError("Only one of --early-only and --not-early-only can be set")

    pipeline_path = Path(pipeline).expanduser()
    if not pipeline_path.exists():
        raise ValueError(f"Pipeline file does not exist: {pipeline}")

    base_steps_path = Path("step.json")
    with open(base_steps_path, "r") as f:
        base_step = json.load(f)

    artifact_destination = os.environ["BUCKET_PATH"] + os.environ["BUILDKITE_COMMIT"]

    assert pipeline
    assert image
    assert queue
    assert early_only ^ not_early_only

    pipeline_steps = read_pipeline(pipeline_path)

    if early_only:
        pipeline_steps = filter_pipeline_conditions(
            pipeline_steps, include=["NO_WHEELS_REQUIRED"]
        )
    elif not_early_only:
        pipeline_steps = filter_pipeline_conditions(
            pipeline_steps, exclude=["NO_WHEELS_REQUIRED"]
        )

    # Todo: Filter affected set conditions

    pipeline_steps = update_steps(pipeline_steps, base_step)

    pipeline_steps = update_steps(
        pipeline_steps,
        {
            "plugins": {"docker#v3.7.0": image},
            "agents": {"queue": queue},
            "env": {"BUILDKITE_ARTIFACT_UPLOAD_DESTINATION": artifact_destination},
        },
    )

    # Todo: Inject command

    pipeline_steps = drop_pipeline_keys(pipeline_steps, ["conditions"])

    # Todo: Print as json
