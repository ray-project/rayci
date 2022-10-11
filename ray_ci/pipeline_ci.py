import collections
import json
import os
from functools import partial
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional

import click
import yaml


EARLY_SETUP_COMMANDS = [
    "echo '--- :running: Early kick-off: Checking out PR code revision'",
    "git remote add pr_repo {repo_url}",
    "git fetch pr_repo {repo_branch}",
    "git checkout pr_repo/{repo_branch}",
    (
        '[[ "$(git log -1 --format="%H")" == "{git_hash}" ]] || '
        '(echo "Quick start failed: Wrong commit hash!" && exit 1)'
    ),
    "BAZEL_CONFIG_ONLY=1 ./ci/env/install-bazel.sh",
    'echo "build --remote_upload_local_results=false" >> /root/.bazelrc',
    "echo 'export PS4=\">\"' >> ~/.bashrc",
]

BASE_STEPS_JSON = Path(__file__).parent / "step.json"


def get_specific_queues():
    return {
        os.environ.get("RUNNER_QUEUE_DEFAULT", "__runner_queue_default"): {
            "tiny": os.environ.get("RUNNER_QUEUE_TINY", "__runner_queue_tiny"),
            "small": os.environ.get("RUNNER_QUEUE_SMALL", "__runner_queue_small"),
            "medium": os.environ.get("RUNNER_QUEUE_MEDIUM", "__runner_queue_medium"),
            "large": os.environ.get("RUNNER_QUEUE_LARGE", "__runner_queue_large"),
        }
    }


def read_pipeline(pipeline_path: Path):
    if not pipeline_path.exists():
        return []

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
        conditions = step.get(key, ["ALWAYS"])
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
        if key.startswith("RAY_CI_") and bool(int(val)):
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


def clean_repo_branch(repo_branch_full: str) -> str:
    # Remove user: from user:branch
    return repo_branch_full.split(":", maxsplit=1)[-1]


def create_setup_commands(repo_url: str, repo_branch: str, git_hash: str) -> List[str]:
    commands = []
    for command in EARLY_SETUP_COMMANDS:
        commands.append(
            command.format(
                repo_url=repo_url, repo_branch=repo_branch, git_hash=git_hash
            )
        )
    return commands


def map_commands(
    steps: List[Dict[str, Any]],
    map_fn: Callable[[str], List[str]],
    key: str = "commands",
):
    steps = steps.copy()
    for step in steps:
        new_vals = []
        for val in step[key]:
            new_vals += map_fn(val)

        step[key] = new_vals
    return steps


def _update_step(
    step: Dict[str, Any], queue: str, image: str, artifact_destination: str
):
    step["plugins"][1]["docker#v3.7.0"]["image"] = image

    queue_to_use = queue

    specific_queues = get_specific_queues()

    # Potentially overwrite with specific queue
    specific_queue_name = step.get("instance_size", None)
    if specific_queue_name:
        new_queue = specific_queues.get(queue, {}).get(specific_queue_name)
        if new_queue:
            queue_to_use = new_queue

    step["agents"]["queue"] = queue_to_use
    step["env"]["BUILDKITE_ARTIFACT_UPLOAD_DESTINATION"] = artifact_destination


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

    artifact_destination = (
        os.environ["BUCKET_PATH"] + "/" + os.environ["BUILDKITE_COMMIT"]
    )

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
    pipeline_steps = update_steps(
        pipeline_steps,
        partial(
            _update_step,
            queue=queue,
            image=image,
            artifact_destination=artifact_destination,
        ),
    )

    # Drop conditions key as it is custom (and not supported by buildkite)
    pipeline_steps = drop_pipeline_keys(pipeline_steps, ["conditions", "instance_size"])

    # Inject print commands
    def _print_command(cmd: str) -> List[str]:
        cmd_str = f"echo '--- :arrow_forward: {cmd}'"
        return [cmd_str, cmd]

    pipeline_steps = map_commands(pipeline_steps, map_fn=_print_command)

    # On early start, inject early setup commands
    if early_only:
        setup_commands = create_setup_commands(
            repo_url=os.environ["BUILDKITE_PULL_REQUEST_REPO"],
            repo_branch=clean_repo_branch(os.environ["BUILDKITE_BRANCH"]),
            git_hash=os.environ["BUILDKITE_COMMIT"],
        )
        pipeline_steps = inject_commands(pipeline_steps, before=setup_commands)

    # Print to stdout
    steps_str = json.dumps(pipeline_steps)
    print(steps_str)


if __name__ == "__main__":
    main()
