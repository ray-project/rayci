#!/bin/bash
set -e

export DOCKER_BUILDKIT=1

cd "$RAY_REPO_DIR" || true

if [ "$BUILDKITE_COMMIT" = "HEAD" ]; then
  export BUILDKITE_COMMIT=$(git log -1 --format="%H")
  echo "Resolved BUILDKITE_COMMIT to $BUILDKITE_COMMIT"
fi

# Convert / into _
if [ -z "${BUILDKITE_PULL_REQUEST_BASE_BRANCH-}" ]; then
  # In branches, use the BUILDKITE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}
else
  # In PRs, use the BUILDKITE_PULL_REQUEST_BASE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_PULL_REQUEST_BASE_BRANCH/\//_}
fi

export DOCKER_IMAGE_BASE_ARM64=$ECR_BASE_REPO:oss-ci-base_arm64_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TAG_ARM64=$ECR_BASE_REPO:oss-ci-base_arm64_latest_$BUILDKITE_BRANCH_CLEAN

echo "--- :docker: Building base dependency image for ARM64 tests :mechanical_arm:"
date +"%Y-%m-%d %H:%M:%S"

# Pass base image
export DOCKER_IMAGE_BASE_UBUNTU=arm64v8/ubuntu:focal

time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_UBUNTU \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BASE_ARM64" \
  -t "$DOCKER_IMAGE_TAG_ARM64" \
  -f ci/docker/base.test.Dockerfile .

date +"%Y-%m-%d %H:%M:%S"

if [ "${NO_PUSH}" = "1" ]; then
  echo "--- :exclamation: Not pushing the image as this is a local build only!"
  exit 0
fi

echo "--- :arrow_up: Pushing docker image to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BASE_ARM64"
time docker push "$DOCKER_IMAGE_TAG_ARM64"

date +"%Y-%m-%d %H:%M:%S"
