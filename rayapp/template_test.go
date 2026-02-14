package rayapp

const testBuildDotYaml = `
# Just for testing

- name: job-intro
  emoji: 🔰
  title: Intro to Jobs
  description: Introduction on how to use Anyscale Jobs
  dir: templates/intro-jobs
  cluster_env:
    build_id: anyscaleray2340-py311
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml

- name: workspace-intro
  emoji: 🔰
  title: Intro to Workspaces
  description: Introduction on how to use Anyscale Workspaces
  dir: templates/intro-workspaces
  cluster_env:
    byod:
      docker_image: cr.ray.io/ray:2340-py311
      ray_version: 2.34.0
  compute_config:
    GCP: configs/basic-single-node/gce.yaml
    AWS: configs/basic-single-node/aws.yaml
`
