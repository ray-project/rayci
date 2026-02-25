# Template Testing Configuration

## Overview

Each template in `BUILD.yaml` must now include a `test` configuration that defines how the template should be tested.

## Test Configuration Fields

- **`command`** (required): The test command to execute in the workspace
- **`timeout_in_sec`** (optional): Maximum time allowed for tests to run (defaults to 3600 seconds)
- **`tests_path`** (optional): Path to a separate test folder to be zipped and pushed to the workspace

## Example BUILD.yaml

### Basic Configuration (command only)

```yaml
- name: my-template
  emoji: 🚀
  title: My Template
  description: A sample template
  dir: templates/my-template
  cluster_env:
    build_id: anyscaleray2370-py311
  compute_config:
    AWS: configs/aws.yaml
  test:
    command: pip install pytest && pytest . -v
```

### Configuration with Custom Timeout

```yaml
- name: long-running-template
  emoji: ⏱️
  title: Long Running Template
  description: Template with extended test time
  dir: templates/long-running-template
  cluster_env:
    build_id: anyscaleray2370-py311
  compute_config:
    AWS: configs/aws.yaml
  test:
    command: pip install pytest && pytest . -v
    timeout_in_sec: 7200  # 2 hours
```

### Configuration with Separate Test Folder

```yaml
- name: template-with-tests
  emoji: 🧪
  title: Template with Test Suite
  description: Template with a separate test directory
  dir: templates/template-with-tests
  cluster_env:
    build_id: anyscaleray2370-py311
  compute_config:
    AWS: configs/aws.yaml
  test:
    command: pip install -r tests/requirements.txt && pytest tests/ -v
    # If the test scripts and artifacts are located at tests/template-with-tests/ci/tests
    tests_path: tests/template-with-tests/ci/  # Everything inside ci/ is copied to workspace
    timeout_in_sec: 1800  # 30 minutes
```

## Test Execution Flow

When running tests, the following steps occur:

1. Create and start an Anyscale workspace
2. Zip the template contents folder
3. Push the template zip to the workspace
4. Unzip the template contents in the workspace
5. If `tests_path` is specified:
   - Zip the contents of the folder at `tests_path`
   - Push the test zip to the workspace
   - Unzip the contents in the workspace
   - **Note**: The folder at `tests_path` will not be copied to workspace. The workspace will only
   contain the contents of the `tests_path` folder.
6. Execute the test `command`
7. Clean up the workspace (terminate and delete)

## Running Tests

```bash
# Test a specific template in default BUILD.yaml
./rayapp test my-template

# Test all templates in default BUILD.yaml
./rayapp test all

# Test a specific template with a custom build file
./rayapp test my-template --build BUILD.yaml

# Test all templates in a custom build file
./rayapp test all --build BUILD.yaml
```

## Migration Guide

If you have existing templates without the `test` configuration, they will be skipped during tests.
To add tests, add tests as shown in the command below, or through a test.sh script that invokes the
required test scripts.

```yaml
test:
  command: pip install nbmake==1.5.5 pytest==9.0.2 && pytest --nbmake . -s -vv
```
