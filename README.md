# Ray Buildkite CI pipeline utilities

This repository contains scripts to run the Ray CI tests on Buildkite.

Specifically, these scripts are called by the Buildkite agent whenever a branch or PR is built.

## Periodic base image build

We set up a periodic pipeline that builds the base images nightly. The scripts that we call
are `build_base_build.sh` and `build_base_gpu.sh`. These scripts build hierarchical docker images
and push them to a common ECR registry.

## Per-commit builds

On every commit, `bootstrap_linux.sh` is run. This script will first try to find and download
existing base images for the respective base branch. If this does not exist, the base images
are built and uploaded first. The first build of a new branch will thus take longer to build.

After that, the script builds the per-commit images on top of these base images and kicks off
the respective pipelines, which are defined in the main Ray repository. The script `pipeline_cy.py`
is used to construct these pipelines.

## Early kick-off

The pipelines support early kick-off. In PR builds, pipeline steps marked with `NO_WHEELS_REQUIRED`
can be run on the latest available branch image if the Ray binaries were not changed. In that
case, these tests are kicked off early, and a git checkout command series is injected before the actual
commands to fetch the respective code revision.
