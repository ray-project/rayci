import collections
import json
import os
from functools import partial
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional

import click
import yaml


# Todo: Early setup commands
EARLY_SETUP_COMMANDS = []

BASE_STEPS_JSON = Path(__file__).parent / "step.json"


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


def update_steps(
    steps: List[Dict[str, Any]], callback: Callable[[Dict[str, Any]], None]
):
    steps = steps.copy()
    for step in steps:
        callback(step)
    return steps


def drop_pipeline_keys(steps: List[Dict[str, Any]], keys: List[str]):
    steps = steps.copy()
    for step in steps:
        for key in keys:
            step.pop(key, None)
    return steps


def get_affected_set_conditions():
    conditions = []
    for key, val in os.environ.items():
        if key.startswith("RAY_CI_"):
            conditions.append(key)
    return ["ALWAYS"] + conditions


def inject_commands(
    steps: List[Dict[str, Any]],
    before: Optional[List[str]] = None,
    after: Optional[List[str]] = None,
    key: str = "commands",
):
    steps = steps.copy()

    before = before or []
    after = after or []

    for step in steps:
        step[key] = before + step[key] + after

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

    if not queue:
        raise ValueError("Please specify a runner queue using --queue")

    if early_only and not_early_only:
        raise ValueError("Only one of --early-only and --not-early-only can be set")

    pipeline_path = Path(pipeline).expanduser()
    if not pipeline_path.exists():
        raise ValueError(f"Pipeline file does not exist: {pipeline}")

    with open(BASE_STEPS_JSON, "r") as f:
        base_step = json.load(f)

    artifact_destination = os.environ["BUCKET_PATH"] + os.environ["BUILDKITE_COMMIT"]

    assert pipeline
    assert image
    assert queue
    assert not (early_only and not_early_only)

    pipeline_steps = read_pipeline(pipeline_path)

    # Filter early kick-off
    if early_only:
        pipeline_steps = filter_pipeline_conditions(
            pipeline_steps, include=["NO_WHEELS_REQUIRED"]
        )
    elif not_early_only:
        pipeline_steps = filter_pipeline_conditions(
            pipeline_steps, exclude=["NO_WHEELS_REQUIRED"]
        )

    # Filter include conditions ("conditions" field in pipeline yamls)
    include_conditions = get_affected_set_conditions()

    pipeline_steps = filter_pipeline_conditions(
        pipeline_steps, include=include_conditions
    )

    # Merge with base step
    pipeline_steps = update_steps(pipeline_steps, partial(deep_update, u=base_step))

    # Merge pipeline/queue-specific settings

    def _update_step(step: Dict[str, Any]):
        step["plugins"][1]["docker#v3.7.0"]["image"] = image
        step["agents"]["queue"] = queue
        step["env"]["BUILDKITE_ARTIFACT_UPLOAD_DESTINATION"] = artifact_destination

    pipeline_steps = update_steps(pipeline_steps, _update_step)

    # Drop conditions key as it is custom (and not supported by buildkite)
    pipeline_steps = drop_pipeline_keys(pipeline_steps, ["conditions"])

    # On early start, inject early setup commands
    if early_only:
        pipeline_steps = inject_commands(pipeline_steps, before=EARLY_SETUP_COMMANDS)

    # Print to stdout
    steps_str = json.dumps(pipeline_steps)
    print(steps_str)


if __name__ == "__main__":
    main()
