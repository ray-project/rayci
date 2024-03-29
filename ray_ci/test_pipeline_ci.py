import json
import os

from pathlib import Path
import pytest
import os
import subprocess
import sys
import tempfile

from typing import List

from pipeline_ci import (
    __file__ as pipeline_ci_file,
    filter_pipeline_conditions,
    inject_commands,
    clean_repo_branch,
    create_setup_commands,
    map_commands,
    DEFAULT_BASE_STEPS_JSON,
    _update_step,
    read_pipeline,
)


def test_filter_pipeline_conditions():
    pipeline_steps = [
        {"name": "a", "conditions": ["A", "B", "C"]},
        {"name": "b", "conditions": ["B", "C"]},
        {"name": "c", "conditions": ["C"]},
        {"name": "d", "conditions": ["D"]},
    ]

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(pipeline_steps, include=["B"])
    ]
    assert "a" in filtered
    assert "b" in filtered
    assert "c" not in filtered
    assert "d" not in filtered

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(pipeline_steps, exclude=["B"])
    ]
    assert "a" not in filtered
    assert "b" not in filtered
    assert "c" in filtered
    assert "d" in filtered

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(pipeline_steps, include=["C"])
    ]
    assert "a" in filtered
    assert "b" in filtered
    assert "c" in filtered
    assert "d" not in filtered

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(
            pipeline_steps, include=["B"], exclude=["A"]
        )
    ]
    assert "a" not in filtered
    assert "b" in filtered
    assert "c" not in filtered
    assert "d" not in filtered

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(
            pipeline_steps, include=["B", "D"], exclude=["A"]
        )
    ]
    assert "a" not in filtered
    assert "b" in filtered
    assert "c" not in filtered
    assert "d" in filtered

    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(
            pipeline_steps, include=["B", "D"], exclude=["A", "D"]
        )
    ]
    assert "a" not in filtered
    assert "b" in filtered
    assert "c" not in filtered
    assert "d" not in filtered


def test_filter_pipeline_conditions_always():
    pipeline_steps = [
        {"name": "a", "conditions": ["A", "B", "C"]},
        {"name": "b"},
        {"name": "c", "conditions": ["C"]},
        {"name": "d", "conditions": ["D"]},
    ]
    filtered = [
        item["name"]
        for item in filter_pipeline_conditions(pipeline_steps, include=["ALWAYS", "C"])
    ]
    assert filtered == ["a", "b", "c"]


def test_inject_commands():
    pipeline_steps = [
        {"name": "a", "commands": ["A", "B", "C"]},
        {"name": "b", "commands": ["B", "C"]},
    ]
    inject_commands(pipeline_steps, before=["X"], after=["Z"])
    assert all(step["commands"][0] == "X" for step in pipeline_steps)
    assert all(step["commands"][-1] == "Z" for step in pipeline_steps)


def test_clean_repo_branch():
    assert clean_repo_branch("bar") == "bar"
    assert clean_repo_branch("foo:bar") == "bar"
    assert clean_repo_branch("foo:bar/boo") == "bar/boo"
    assert clean_repo_branch("foo:bar:boo") == "bar:boo"


def test_create_setup_commands():
    commands = create_setup_commands(
        repo_url="SOME_URL", repo_branch="SOME_BRANCH", git_hash="abcd1234"
    )
    cmds_before_git = 4

    assert commands[-cmds_before_git - 4] == "git remote add pr_repo SOME_URL"
    assert commands[-cmds_before_git - 3] == "git fetch pr_repo SOME_BRANCH"
    assert commands[-cmds_before_git - 2] == "git checkout pr_repo/SOME_BRANCH"
    assert "abcd1234" in commands[-cmds_before_git - 1]
    assert commands[-cmds_before_git].startswith(
        "git checkout master python/ray/_raylet.pxd python/ray/_raylet.pyi"
    )


def test_pipeline_map_steps():
    def _print_command(cmd: str) -> List[str]:
        cmd_str = f"echo --- :arrow_forward: {cmd}"
        return [cmd_str, cmd]

    assert map_commands([{"commands": ["A", "B"]}], map_fn=_print_command) == [
        {
            "commands": [
                "echo --- :arrow_forward: A",
                "A",
                "echo --- :arrow_forward: B",
                "B",
            ]
        }
    ]


def test_pipeline_update_queue():
    queue = "queue_default"
    small_queue = "queue_small"

    with open(DEFAULT_BASE_STEPS_JSON, "r") as f:
        base_step = json.load(f)

    # Changes to env
    os.environ["RUNNER_QUEUE_DEFAULT"] = queue
    os.environ["RUNNER_QUEUE_SMALL"] = small_queue

    # No changes
    step = base_step.copy()

    _update_step(step, queue=queue, image="", artifact_destination="")

    assert step["agents"]["queue"] == queue

    # small instance size
    step = base_step.copy()
    step["instance_size"] = "small"

    _update_step(step, queue=queue, image="", artifact_destination="")

    assert step["agents"]["queue"] == small_queue

    step = base_step.copy()
    step["instance_size"] = "medium"

    with pytest.raises(ValueError):
        _update_step(step, queue=queue, image="", artifact_destination="")

    # invalid instance size
    step = base_step.copy()
    step["instance_size"] = "invalid"

    with pytest.raises(ValueError):
        _update_step(step, queue=queue, image="", artifact_destination="")

    # Cleanup
    os.environ.pop("RUNNER_QUEUE_DEFAULT", None)
    os.environ.pop("RUNNER_QUEUE_SMALL", None)


def test_read_lineline():
    with tempfile.TemporaryDirectory() as tmpdir:
        pipeline_path = Path(os.path.join(tmpdir, "pipeline.yml"))
        with open(pipeline_path, "w") as f:
            f.write("#ci:group=foo")
        steps, group_name = read_pipeline(pipeline_path)
        assert group_name == "foo"
        assert steps == []

    with tempfile.TemporaryDirectory() as tmpdir:
        pipeline_path = Path(os.path.join(tmpdir, "pipeline.yml"))
        with open(pipeline_path, "w") as f:
            f.write("- name: foo")
        steps, group_name = read_pipeline(pipeline_path)
        assert group_name is None
        assert steps == [{"name": "foo"}]

    with tempfile.TemporaryDirectory() as tmpdir:
        pipeline_path = Path(os.path.join(tmpdir, "pipeline.yml"))
        with open(pipeline_path, "w") as f:
            f.write("steps:\n  - name: foo\n")
        steps, group_name = read_pipeline(pipeline_path)
        assert group_name is None
        assert steps == [{"name": "foo"}]

    with tempfile.TemporaryDirectory() as tmpdir:
        pipeline_path = Path(os.path.join(tmpdir, "pipeline.yml"))
        with open(pipeline_path, "w") as f:
            f.write("#ci:group=foo\nsteps:\n  - name: foo\n")
        steps, group_name = read_pipeline(pipeline_path)
        assert group_name == "foo"
        assert steps == [{"name": "foo"}]


def test_pipeline_ci():
    env = os.environ.copy()
    env.update({
        "REMOTE_CACHE_URL": "https://remote-cache",
        "ECR_REPO": "ecr_repo",
        "ECR_BASE_REPO": "ecr_base_repo",
        "RUNNER_QUEUE_DEFAULT": "default",
        "RUNNER_QUEUE_SMALL": "small",
        "RUNNER_QUEUE_MEDIUM": "medium",
        "RUNNER_QUEUE_LARGE": "large",
        "RUNNER_QUEUE_GPU_NORM": "gpu",
        "RUNNER_QUEUE_GPU_LARGE": "gpularge",
        "BUCKET_PATH": "s3://bucket",
        "RAY_REPO_DIR": "/ray",
        "PIPELINE_REPO_DIR": "/pipeline",

        "RAY_CI_PYTHON_DEPENDENCIES_AFFECTED": "1",
        "BUILDKITE_PULL_REQUEST_REPO": "ray",
        "BUILDKITE_BRANCH": "master",
        "BUILDKITE_COMMIT": "abcd1234",

        "RAY_CI_JAVA_AFFECTED": "1",
        "RAY_CI_TRAIN_AFFECTED": "1",
    })

    dir = os.path.dirname(__file__)
    pipeline_path = os.path.join(dir, "testdata/pipeline.yml")
    upload = subprocess.check_output(
        [
            sys.executable, pipeline_ci_file,
            "--image", "base_img",
            "--queue", "q",
            pipeline_path,
        ],
        env=env,
    )

    parsed = json.loads(upload)
    assert len(parsed) == 1
    group = parsed[0]
    assert group["group"] == "build"
    assert len(list(group["steps"])) == 3


if __name__ == "__main__":
    sys.exit(pytest.main(["-v", __file__]))
