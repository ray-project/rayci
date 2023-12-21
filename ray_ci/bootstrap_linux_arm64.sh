#!/bin/bash
set -e

if [[ -f .buildkite/.sunset_civ1_linux ]]; then
  echo "Skipping legacy CIv1."
  exit 0
fi

# --- BUILD image

echo "--- :arrow_down: Pulling pre-built BASE ARM64 image"
date +"%Y-%m-%d %H:%M:%S"

BUILD_OWN_BASE="${BUILD_OWN_BASE-0}"
if [[ "$(time docker pull "$DOCKER_IMAGE_BASE_ARM64")" ]]; then
  echo "Pre-built image found: $DOCKER_IMAGE_BASE_ARM64"
else
  # No pre-built image, so we have to build ourselves!
  echo "Pre-built image NOT found: $DOCKER_IMAGE_BASE_ARM64"
  BUILD_OWN_BASE=1
fi

if [[ "$BUILD_OWN_BASE" == "1" ]]; then
  echo "--- :exclamation: No pre-built image found, building ourselves!"
  bash "${PIPELINE_REPO_DIR}/ray_ci/build_base_arm64.sh"
fi

echo "--- :docker: Building docker image ARM64 with compiled Ray :gear:"
date +"%Y-%m-%d %H:%M:%S"

# Overwrite base image
time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_BUILD="$DOCKER_IMAGE_BASE_ARM64" \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_ARM64" \
  -t "$DOCKER_IMAGE_LATEST_ARM64" \
  -f ci/docker/build.Dockerfile .

# --- BUILD pipeline

echo "--- :arrow_up: Pushing docker image ARM64 to ECR :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_ARM64"

# Only push latest images for branch builds
if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
  time docker push "$DOCKER_IMAGE_LATEST_ARM64"
fi

echo "--- :rocket: Launching ARM64 tests :gear:"
echo "Kicking off the full ARM64 pipeline"

python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_ARM64" --queue "$RUNNER_QUEUE_ARM64_MEDIUM" \
  "./.buildkite/pipeline.arm64.yml" | buildkite-agent pipeline upload
