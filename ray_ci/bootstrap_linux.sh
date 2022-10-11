#!/bin/bash
set -e

export DOCKER_BUILDKIT=1

cd "$RAY_REPO_DIR" || true

if [ "$BUILDKITE_COMMIT" = "HEAD" ]; then
  export BUILDKITE_COMMIT=$(git log -1 --format="%H")
  echo "Resolved BUILDKITE_COMMIT to $BUILDKITE_COMMIT"
fi

echo Just a test

exit 0

if [[ "$BUILDKITE_MESSAGE" =~ "[build_base]" ]]; then
   echo "Got build base trigger - rebuilding base images!"
   export BUILD_OWN_BASE="1"
   export BUILD_OWN_GPU="1"
   export NO_PUSH="1"
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

export DOCKER_IMAGE_BUILD=$ECR_BASE_REPO:oss-ci-build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TEST=$ECR_BASE_REPO:oss-ci-test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_ML=$ECR_BASE_REPO:oss-ci-ml_$BUILDKITE_COMMIT
export DOCKER_IMAGE_GPU=$ECR_BASE_REPO:oss-ci-gpu_$BUILDKITE_COMMIT

export DOCKER_IMAGE_LATEST_BUILD=$ECR_BASE_REPO:oss-ci-build_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_TEST=$ECR_BASE_REPO:oss-ci-test_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_ML=$ECR_BASE_REPO:oss-ci-ml_latest_$BUILDKITE_BRANCH_CLEAN
export DOCKER_IMAGE_LATEST_GPU=$ECR_BASE_REPO:oss-ci-gpu_latest_$BUILDKITE_BRANCH_CLEAN

export EARLY_IMAGE_TEST=$ECR_BASE_REPO:oss-ci-test_latest_$BUILDKITE_BRANCH_CLEAN
export EARLY_IMAGE_ML=$ECR_BASE_REPO:oss-ci-ml_latest_$BUILDKITE_BRANCH_CLEAN
export EARLY_IMAGE_GPU=$ECR_BASE_REPO:oss-ci-gpu_latest_$BUILDKITE_BRANCH_CLEAN

python3 -m pip install -U click pyyaml

echo "--- :alarm_clock: Determine if we should kick-off some steps early"

# Fix: path to ray repo
export $(python3 ci/pipeline/determine_tests_to_run.py)


# On pull requests, allow to run on latest available image if wheels are not affected
if [ "${BUILDKITE_PULL_REQUEST}" != "false" ] && [ "$RAY_CI_CORE_CPP_AFFECTED" != "1" ] && [ "$RAY_CI_PYTHON_DEPENDENCIES_AFFECTED" != "1" ]; then
  export KICK_OFF_EARLY=1
  echo "Kicking off some tests early, as this is a PR, and the core C++ is not affected, and requirements are not affected. "
else
  export KICK_OFF_EARLY=0
  echo "This is a branch build (PR=${BUILDKITE_PULL_REQUEST}) or C++ is affected (affected=$RAY_CI_CORE_CPP_AFFECTED), or requirements are affected (affected=$RAY_CI_PYTHON_DEPENDENCIES_AFFECTED). "
  echo "We can't kick off tests early."
fi

if [ "${KICK_OFF_EARLY}" = "1" ]; then
  echo "--- :running: Kicking off some tests early"

  if [[ "$(docker manifest inspect $EARLY_IMAGE_TEST)" ]]; then
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE_TEST" --queue "$RUNNER_QUEUE_DEFAULT" \
      "./.buildkite/pipeline.test.yml" | buildkite-agent pipeline upload
  else
    echo "Docker image NOT FOUND for early test kick-off TEST: $EARLY_IMAGE_TEST"
  fi

  if [[ "$(docker manifest inspect $EARLY_IMAGE_ML)" ]]; then
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE_ML" --queue "$RUNNER_QUEUE_DEFAULT" \
      "./.buildkite/pipeline.ml.yml" | buildkite-agent pipeline upload
  else
      echo "Docker image NOT FOUND for early test kick-off ML: $EARLY_IMAGE_ML"
  fi

  if [[ "$(docker manifest inspect $EARLY_IMAGE_GPU)" ]]; then
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_NORM" \
      "./.buildkite/pipeline.gpu.yml" | buildkite-agent pipeline upload
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_LARGE" \
      "./.buildkite/pipeline.gpu_large.yml" | buildkite-agent pipeline upload
  else
      echo "Docker image NOT FOUND for early test kick-off GPU: $EARLY_IMAGE_GPU"
  fi
fi

# --- BUILD image

echo "--- :arrow_down: Pulling pre-built BASE BUILD image"
date +"%Y-%m-%d %H:%M:%S"

BUILD_OWN_BASE="${BUILD_OWN_BASE-0}"
if [[ "$(time docker pull "$DOCKER_IMAGE_BASE_BUILD")" ]]; then
  echo "Pre-built image found: $DOCKER_IMAGE_BASE_BUILD"
else
  # No pre-built image, so we have to build ourselves!
  echo "Pre-built image NOT found: $DOCKER_IMAGE_BASE_BUILD"
  BUILD_OWN_BASE=1
fi

if [ "$BUILD_OWN_BASE" = "1" ]; then
  echo "--- :exclamation: No pre-built image found, building ourselves!"
  bash "${PIPELINE_REPO_DIR}/ray_ci/build_base_build.sh"
fi

echo "--- :docker: Building docker image BUILD with compiled Ray :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_BUILD \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BUILD" \
  -t "$DOCKER_IMAGE_LATEST_BUILD" \
  -f ci/docker/build.Dockerfile .


# --- BUILD pipeline

echo "--- :arrow_up: Pushing docker image BUILD to ECR :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BUILD"

# Only push latest images for branch builds
if [ "${BUILDKITE_PULL_REQUEST}" = "false" ]; then
  time docker push "$DOCKER_IMAGE_LATEST_BUILD"
fi

echo "--- :rocket: Launching BUILD tests :gear:"
echo "Kicking off the full BUILD pipeline"

python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_BUILD" --queue "$RUNNER_QUEUE_DEFAULT" \
  "./.buildkite/pipeline.build.yml" | buildkite-agent pipeline upload


# --- extract compiled Ray
echo "--- :chopsticks: Extracting compiled Ray from image"

rm -rf /tmp/extracted_ray
CID=$(docker create $DOCKER_IMAGE_BUILD)
echo Docker extraction container ID: $CID
docker cp $CID:/ray/ /tmp/extracted_ray
docker rm $CID
cp -rf /tmp/extracted_ray/* ./
rm -rf /tmp/extracted_ray

# --- TEST image + pipeline

if [ "${BUILD_OWN_BASE-}" != "1" ]; then
  echo "--- :arrow_down: Pulling pre-built BASE TEST image"
  date +"%Y-%m-%d %H:%M:%S"
  time docker pull "$DOCKER_IMAGE_BASE_TEST"
fi

echo "--- :docker: Building docker image TEST for regular CI tests :python:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_TEST" \
  -t "$DOCKER_IMAGE_LATEST_TEST" \
  -f ci/docker/test.Dockerfile .

echo "--- :arrow_up: Pushing docker image TEST to ECR :python:"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_TEST"

# Only push latest images for branch builds
if [ "${BUILDKITE_PULL_REQUEST}" = "false" ]; then
  time docker push "$DOCKER_IMAGE_LATEST_TEST"
fi

echo "--- :rocket: Launching TEST tests :python:"

if [ "${KICK_OFF_EARLY}" = "1" ]; then
  echo "Kicking off the rest of the TEST pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --not-early-only --image "$DOCKER_IMAGE_TEST" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.test.yml" | buildkite-agent pipeline upload
else
  echo "Kicking off the full TEST pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_TEST" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.test.yml" | buildkite-agent pipeline upload
fi

# --- ML image + pipeline

if [ "${BUILD_OWN_BASE-}" != "1" ]; then
  echo "--- :arrow_down: Pulling pre-built BASE ML image"
  date +"%Y-%m-%d %H:%M:%S"
  time docker pull "$DOCKER_IMAGE_BASE_ML"
fi

echo "--- :docker: Building docker image ML with ML dependencies :airplane:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_ML \
  -t "$DOCKER_IMAGE_ML" \
  -t "$DOCKER_IMAGE_LATEST_ML" \
  -f ci/docker/ml.Dockerfile .

echo "--- :arrow_up: Pushing docker image ML to ECR :airplane:"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_ML"

# Only push latest images for branch builds
if [ "${BUILDKITE_PULL_REQUEST}" = "false" ]; then
  time docker push "$DOCKER_IMAGE_LATEST_ML"
fi

echo "--- :rocket: Launching ML tests :airplane:"

if [ "${KICK_OFF_EARLY}" = "1" ]; then
  echo "Kicking off the rest of the ML pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --not-early-only --image "$DOCKER_IMAGE_ML" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.ml.yml" | buildkite-agent pipeline upload
else
  echo "Kicking off the full ML pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_ML" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.ml.yml" | buildkite-agent pipeline upload
fi

# --- GPU image + pipeline

echo "--- :arrow_down: Pulling pre-built BASE GPU image"
date +"%Y-%m-%d %H:%M:%S"

BUILD_OWN_GPU="${BUILD_OWN_GPU-0}"
if [[ "$(time docker pull "$DOCKER_IMAGE_BASE_GPU")" ]]; then
  echo "Pre-built image found: $DOCKER_IMAGE_BASE_GPU"
else
  # No pre-built image, so we have to build ourselves!
  echo "Pre-built image NOT found: $DOCKER_IMAGE_BASE_GPU"
  BUILD_OWN_GPU=1
fi

if [ "$BUILD_OWN_GPU" = "1" ]; then
  echo "--- :exclamation: No pre-built image found, building ourselves!"
  bash "${PIPELINE_REPO_DIR}/ray_ci/build_base_gpu.sh"
fi

echo "--- :docker: Building docker image GPU with ML dependencies :tv:"
date +"%Y-%m-%d %H:%M:%S"

time docker build --progress=plain \
  --build-arg DOCKER_IMAGE_BASE_GPU \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_GPU" \
  -t "$DOCKER_IMAGE_LATEST_GPU" \
  -f ci/docker/gpu.Dockerfile .

echo "--- :arrow_up: Pushing docker image GPU to ECR :tv:"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_GPU"

# Only push latest images for branch builds
if [ "${BUILDKITE_PULL_REQUEST}" = "false" ]; then
  time docker push "$DOCKER_IMAGE_LATEST_GPU"
fi

echo "--- :rocket: Launching GPU tests :tv:"

if [ "${KICK_OFF_EARLY}" = "1" ]; then
  echo "Kicking off the rest of the GPU pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --not-early-only --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_NORM" \
    "./.buildkite/pipeline.gpu.yml" | buildkite-agent pipeline upload
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --not-early-only --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_LARGE" \
    "./.buildkite/pipeline.gpu_large.yml" | buildkite-agent pipeline upload
else
  echo "Kicking off the full GPU pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_NORM" \
    "./.buildkite/pipeline.gpu.yml" | buildkite-agent pipeline upload
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_LARGE" \
    "./.buildkite/pipeline.gpu_large.yml" | buildkite-agent pipeline upload
fi
