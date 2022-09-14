set -xe

echo "--- :docker: Building base dependency image"
date +"%Y-%m-%d %H:%M:%S"

time docker build
  --build-arg REMOTE_CACHE_URL
  --build-arg BUILDKITE_PULL_REQUEST
  --build-arg BUILDKITE_COMMIT
  --build-arg BUILDKITE_PULL_REQUEST_BASE_BRANCH
  -t $ECR_REPO/oss-ci-base:$BUILDKITE_COMMIT
  -t $ECR_REPO/oss-ci-base:latest
  -f ci/docker/Dockerfile.base .

date +"%Y-%m-%d %H:%M:%S"

echo "--- :arrow-up: Pushing docker image to ECR"
date +"%Y-%m-%d %H:%M:%S"

time docker push --all-tags $ECR_REPO:$BUILDKITE_COMMIT

date +"%Y-%m-%d %H:%M:%S"
