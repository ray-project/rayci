import pytest
import sys

from typing import List

from pipeline_ci import (
    filter_pipeline_conditions,
    inject_commands,
    clean_repo_branch,
    create_setup_commands,
    map_commands,
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
    assert commands[1] == "git remote add pr_repo SOME_URL"
    assert commands[2] == "git fetch pr_repo SOME_BRANCH"
    assert commands[3] == "git checkout pr_repo/SOME_BRANCH"
    assert "abcd1234" in commands[3]


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


if __name__ == "__main__":
    sys.exit(pytest.main(["-v", __file__]))
