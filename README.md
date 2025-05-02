This repository contains scripts and progresm for Ray CI/CD on buildkite.

- `rayci`: program that reads test definition files and generates buildkite
  pipeline definitions
- `wanda`: program that builds container images using a container registry as
  a content-addressed build cache

This repository also contains scripts that are used on legacy CI/CD pipelines
and KubeRay pipelines, in the `ray_ci` directory and `ecosystem_ci` directory.
