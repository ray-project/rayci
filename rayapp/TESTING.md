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
  dir: my-template
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
  dir: long-running-template
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
  dir: template-with-tests
  cluster_env:
    build_id: anyscaleray2370-py311
  compute_config:
    AWS: configs/aws.yaml
  test:
    command: pip install -r tests/requirements.txt && pytest tests/ -v
    tests_path: template-with-tests/tests
    timeout_in_sec: 1800  # 30 minutes
```

## Test Execution Flow

When running tests, the following steps occur:

1. Create and start an Anyscale workspace
2. Zip the template contents folder
3. Push the template zip to the workspace
4. Unzip the template contents in the workspace
5. If `tests_path` is specified:
   - Zip the test folder
   - Push the test zip to the workspace
   - Unzip the test folder in the workspace
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
./rayapp test my-template --build BUILD.yaml
```

## Migration Guide

If you have existing templates without the `test` configuration, you need to add it. For templates that were previously using the default pytest command, add:

```yaml
test:
  command: pip install nbmake==1.5.5 pytest==9.0.2 && pytest --nbmake . -s -vv
```
