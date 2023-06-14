#!/bin/bash

export REMOTE_CACHE_URL=s3://remote-cache
export ECR_REPO=ecr_repo
export ECR_BASE_REPO=ecr_base_repo
export RUNNER_QUEUE_DEFAULT=default
export RUNNER_QUEUE_SMALL=small
export RUNNER_QUEUE_MEDIUM=medium
export RUNNER_QUEUE_LARGE=large
export RUNNER_QUEUE_GPU_NORM=gpu
export RUNNER_QUEUE_GPU_LARGE=gpularge
export BUCKET_PATH=s3://bucket
export RAY_REPO_DIR=/ray
export PIPELINE_REPO_DIR=/pipeline

export RAY_CI_PYTHON_DEPENDENCIES_AFFECTED=1234
export BUILDKITE_PULL_REQUEST_REPO=ray
export BUILDKITE_BRANCH=master
export BUILDKITE_COMMIT="abc123"

python pipeline_ci.py --image base_img --queue q ./pipeline.build.yml | jq
