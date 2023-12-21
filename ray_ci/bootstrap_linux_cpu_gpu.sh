#!/bin/bash
set -e

if [[ -f .buildkite/.sunset_civ1_linux && "${RAY_CI_RLLIB_CONTRIB_AFFECTED:-}" != "1" ]]; then
  # We only run legacy CIv1 pipeline when RLlib or contrib marked as affected.
  echo "Skipping legacy CIv1."
  exit 0
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

if [[ "$BUILD_OWN_BASE" == "1" ]]; then
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
if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
  time docker push "$DOCKER_IMAGE_LATEST_BUILD"
fi

echo "--- :rocket: Launching BUILD tests :gear:"
echo "Kicking off the full BUILD pipeline"

if [[ "$(find .buildkite -name 'pipeline.build*.yml')" != "" ]]; then
  BUILD_YAMLS=(.buildkite/pipeline.build*.yml)
  for FILE in "${BUILD_YAMLS[@]}"; do
    python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" \
      --image "$DOCKER_IMAGE_BUILD" --queue "$RUNNER_QUEUE_DEFAULT" \
      "$FILE" | buildkite-agent pipeline upload
  done
fi


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

if [[ -f .buildkite/pipeline.test.yml ]]; then
  echo "Kicking off the TEST pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_TEST" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.test.yml" | buildkite-agent pipeline upload
fi

# --- ML image + pipeline

if [[ "${BUILD_OWN_BASE-}" != "1" ]]; then
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
if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
  time docker push "$DOCKER_IMAGE_LATEST_ML"
fi

echo "--- :rocket: Launching ML tests :airplane:"

if [[ -f .buildkite/pipeline.ml.yml ]]; then
  echo "Kicking off the ML pipeline"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_ML" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.ml.yml" | buildkite-agent pipeline upload
fi

# --- GPU image + pipeline

if [[ ! -e .buildkite/pipeline.gpu.yml && ! -e .buildkite/pipeline.gpu_large.yml ]]; then
  echo "No GPU tests found, skipping GPU pipeline"
  exit 0
fi

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

if [[ "$BUILD_OWN_GPU" == "1" ]]; then
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
if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
  time docker push "$DOCKER_IMAGE_LATEST_GPU"
fi

echo "--- :rocket: Launching GPU tests :tv:"

echo "Kicking off the GPU pipeline"
if [[ -e .buildkite/pipeline.gpu.yml ]]; then
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_NORM" \
    "./.buildkite/pipeline.gpu.yml" | buildkite-agent pipeline upload
fi
if [[ -e .buildkite/pipeline.gpu_large.yml ]]; then
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_GPU" --queue "$RUNNER_QUEUE_GPU_LARGE" \
    "./.buildkite/pipeline.gpu_large.yml" | buildkite-agent pipeline upload
fi
