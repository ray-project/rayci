set -e

cd "$RAY_REPO_DIR" || true

# Export some docker image names
export DOCKER_IMAGE_BASE_BUILD=$ECR_BASE_REPO:oss-ci-base_build_latest_ci_docker
export DOCKER_IMAGE_BASE_TEST=$ECR_BASE_REPO:oss-ci-base_test_latest_ci_docker
# Todo: latest_master
export DOCKER_IMAGE_BUILD=$ECR_REPO:oss-ci-build_$BUILDKITE_COMMIT
export DOCKER_IMAGE_TEST=$ECR_REPO:oss-ci-test_$BUILDKITE_COMMIT
export DOCKER_IMAGE_ML=$ECR_REPO:oss-ci-ml_$BUILDKITE_COMMIT
export DOCKER_IMAGE_GPU=$ECR_REPO:oss-ci-gpu_$BUILDKITE_COMMIT
export EARLY_IMAGE=$ECR_REPO:oss-ci-test_latest-master

python3 -m pip install -U click pyyaml

echo "--- :alarm_clock: Determine if we should kick-off some steps early"

# Fix: path to ray repo
export $(python3 ci/pipeline/determine_tests_to_run.py)

# On pull requests, allow to run on latest available image if wheels are not affected
if [ "${BUILDKITE_PULL_REQUEST}" != "false" ] && [ "$RAY_CI_CORE_CPP_AFFECTED" != "1" ]; then
  export KICK_OFF_EARLY=1
  echo "Kicking off some tests early, as this is a PR, and the core C++ is not affected. "
else
  export KICK_OFF_EARLY=0
  echo "This is a branch build (PR=${BUILDKITE_PULL_REQUEST}) or C++ is affected (affected=$RAY_CI_CORE_CPP_AFFECTED). "
  echo "We can't kick off tests early."
fi

if [ "${KICK_OFF_EARLY}" = "1" ]; then
  echo "--- :running: Kicking off some tests early"
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.test.yml" | buildkite-agent pipeline upload
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE" --queue "$RUNNER_QUEUE_DEFAULT" \
    "./.buildkite/pipeline.ml.yml" | buildkite-agent pipeline upload
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE" --queue "$RUNNER_QUEUE_GPU_NORM" \
    "./.buildkite/pipeline.gpu.yml" | buildkite-agent pipeline upload
  python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --early-only --image "$EARLY_IMAGE" --queue "$RUNNER_QUEUE_GPU_LARGE" \
    "./.buildkite/pipeline.gpu_large.yml" | buildkite-agent pipeline upload
fi

# --- BUILD image

echo "--- :arrow_down: Pulling pre-built BASE BUILD image"
date +"%Y-%m-%d %H:%M:%S"
time docker pull "$DOCKER_IMAGE_BASE_BUILD"

echo "--- :docker: Building docker image BUILD with compiled Ray :gear:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg DOCKER_IMAGE_BASE_BUILD \
  --build-arg REMOTE_CACHE_URL \
  --build-arg BUILDKITE_PULL_REQUEST \
  --build-arg BUILDKITE_COMMIT \
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH \
  -t "$DOCKER_IMAGE_BUILD" \
  -f ci/docker/Dockerfile.build .

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

echo "--- :arrow_down: Pulling pre-built BASE TEST image"
date +"%Y-%m-%d %H:%M:%S"
time docker pull "$DOCKER_IMAGE_BASE_TEST"

echo "--- :docker: :python: Building docker image TEST for regular CI tests"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg DOCKER_IMAGE_BASE_TEST \
  -t "$DOCKER_IMAGE_TEST" \
  -f ci/docker/Dockerfile.test .

echo "--- :arrow_up: :python: Pushing Build docker image TEST to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_TEST"

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

echo "--- :docker: Building docker image ML with ML dependencies :airplane:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg DOCKER_IMAGE_TEST \
  -t "$DOCKER_IMAGE_ML" \
  -f ci/docker/Dockerfile.ml .

echo "--- :arrow_up: :airplane: Pushing Build docker image TEST to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_ML"

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
time docker pull "$DOCKER_IMAGE_BASE_GPU"

echo "--- :docker: Building docker image GPU with ML dependencies :tv:"
date +"%Y-%m-%d %H:%M:%S"

time docker build \
  --build-arg $DOCKER_IMAGE_BASE_GPU \
  -t "$DOCKER_IMAGE_GPU" \
  -f ci/docker/Dockerfile.gpu .

echo "--- :arrow_up: :tv: Pushing Build docker image TEST to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_GPU"

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

# --- BUILD pipeline

echo "--- :arrow_up: :gear: Pushing Build docker image to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push "$DOCKER_IMAGE_BUILD"

python3 "${PIPELINE_REPO_DIR}/ray_ci/pipeline_ci.py" --image "$DOCKER_IMAGE_BUILD" "./.buildkite/pipeline.build.yml" | buildkite-agent pipeline upload
