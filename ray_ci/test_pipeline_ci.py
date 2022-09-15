import pytest
import sys

from pipeline_ci import filter_pipeline_conditions, inject_commands


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


if __name__ == "__main__":
    sys.exit(pytest.main(["-v", __file__]))
