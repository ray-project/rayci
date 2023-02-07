#!/bin/bash
set -e

export DOCKER_BUILDKIT=1

cd "$RAY_REPO_DIR" || true

if [ "$BUILDKITE_COMMIT" = "HEAD" ]; then
  export BUILDKITE_COMMIT=$(git log -1 --format="%H")
  echo "Resolved BUILDKITE_COMMIT to $BUILDKITE_COMMIT"
fi

# Commit message instructions

if [[ "$BUILDKITE_MESSAGE" =~ "[build_base]" ]]; then
   echo "Got build base trigger - rebuilding base images!"
   export BUILD_OWN_BASE="1"
   export BUILD_OWN_GPU="1"
   export NO_PUSH="1"
   export KICK_OFF_EARLY="0"
fi

if [[ "$BUILDKITE_MESSAGE" =~ "[all_tests]" ]]; then
   echo "Got all tests trigger - running all tests!"
   export ALL_TESTS="1"
fi

if [[ "$BUILDKITE_MESSAGE" =~ "[no_early_kickoff]" ]]; then
   echo "Got no early kickoff trigger - preventing early kickoff!"
   export KICK_OFF_EARLY="0"
fi

# Convert / into _
if [ -z "${BUILDKITE_PULL_REQUEST_BASE_BRANCH-}" ]; then
  # In branches, use the BUILDKITE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_BRANCH/\//_}
else
  # In PRs, use the BUILDKITE_PULL_REQUEST_BASE_BRANCH
  export BUILDKITE_BRANCH_CLEAN=${BUILDKITE_PULL_REQUEST_BASE_BRANCH/\//_}
fi

# Export some docker image names
export DOCKER_IMAGE_BASE_BUILD=$ECR_BASE_REPO:oss-ci-base_build_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_BASE_TEST=$ECR_BASE_REPO:oss-ci-base_test_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_BASE_ML=$ECR_BASE_REPO:oss-ci-base_ml_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_BASE_GPU=$ECR_BASE_REPO:oss-ci-base_gpu_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_BASE_ARM64=$ECR_BASE_REPO:oss-ci-base_arm64_latest_$BUILDKITE_BRANCH_CLEAN

export DOCKER_IMAGE_BUILD=$ECR_BASE_REPO:oss-ci-build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TEST=$ECR_BASE_REPO:oss-ci-test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_ML=$ECR_BASE_REPO:oss-ci-ml_$BUILDKITE_COMMIT
export DOCKER_IMAGE_GPU=$ECR_BASE_REPO:oss-ci-gpu_$BUILDKITE_COMMIT
export DOCKER_IMAGE_ARM64=$ECR_BASE_REPO:oss-ci-arm64_$BUILDKITE_COMMIT

export DOCKER_IMAGE_LATEST_BUILD=$ECR_BASE_REPO:oss-ci-build_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_TEST=$ECR_BASE_REPO:oss-ci-test_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_ML=$ECR_BASE_REPO:oss-ci-ml_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_GPU=$ECR_BASE_REPO:oss-ci-gpu_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_ARM64=$ECR_BASE_REPO:oss-ci-arm64_latest_$BUILDKITE_BRANCH_CLEAN

export EARLY_IMAGE_TEST=$ECR_BASE_REPO:oss-ci-test_latest_$BUILDKITE_BRANCH_CLEAN
export EARLY_IMAGE_ML=$ECR_BASE_REPO:oss-ci-ml_latest_$BUILDKITE_BRANCH_CLEAN
export EARLY_IMAGE_GPU=$ECR_BASE_REPO:oss-ci-gpu_latest_$BUILDKITE_BRANCH_CLEAN

python3 -m pip install -U click pyyaml

# Fix: path to ray repo
export $(python3 ci/pipeline/determine_tests_to_run.py)
env

if [ "$HOSTTYPE" = "aarch64" ]; then
  echo "Running ARM64 pipeline"
  bash "${PIPELINE_REPO_DIR}/ray_ci/bootstrap_linux_arm64.sh"
else
  echo "Running CPU/GPU pipeline"
  bash "${PIPELINE_REPO_DIR}/ray_ci/bootstrap_linux_cpu_gpu.sh"
fi
