# Python Development: Using Claudio with Python Projects

**Time: 20-30 minutes**

This tutorial explains how to use Claudio effectively with Python projects, covering virtual environment management, testing strategies, package management, and framework-specific considerations.

## Overview

Claudio's git worktree architecture requires special consideration for Python projects:

- **Virtual environment isolation**: Each worktree needs its own virtualenv
- **Package caching**: pip cache is shared, speeding up installs
- **Test isolation**: pytest runs independently per worktree
- **Framework flexibility**: Works with Django, Flask, FastAPI, and more
- **Data science ready**: Supports Jupyter notebooks and ML workflows

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Python 3.8+ installed
- pip, virtualenv, or uv installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Python and Git Worktrees

### Virtual Environment Challenges

Unlike Node.js or Go, Python virtual environments are **path-dependent**. A virtualenv created at one path won't work at another:

```
Main repo:
└── .venv/  (created at /project/.venv)

.claudio/worktrees/abc123/
└── .venv/  (must be created separately)
```

This means:
- Each worktree needs its own virtualenv
- You cannot share virtualenvs between worktrees
- First activation in each worktree requires `pip install`

### pip Cache Sharing

pip uses a global cache that's shared across all virtualenvs:

| Location | Platform | Shared? |
|----------|----------|---------|
| `~/.cache/pip` | Linux | Yes |
| `~/Library/Caches/pip` | macOS | Yes |
| `%LOCALAPPDATA%\pip\Cache` | Windows | Yes |

**Key insight**: While virtualenvs are isolated, downloaded packages are cached globally, making subsequent installs much faster.

### Installation Times

| Project Size | Fresh Install | With Cache |
|-------------|---------------|------------|
| Small (~20 deps) | 30s - 1min | 10-20s |
| Medium (~50 deps) | 1-2min | 30-45s |
| Large (~100 deps) | 2-5min | 1-2min |
| Data Science | 5-15min | 2-5min |

## Strategy 1: Per-Worktree Virtualenv (Recommended)

Best for: Most Python projects.

### Setup Pattern

Each task should include virtualenv creation:

```
Implement the user authentication module in src/auth/.

Setup:
python -m venv .venv
source .venv/bin/activate  # or .venv\Scripts\activate on Windows
pip install -r requirements.txt

Then:
1. Create src/auth/service.py with login/register logic
2. Create src/auth/models.py with User model
3. Run tests: pytest tests/auth/
```

### Using uv (Recommended for Speed)

[uv](https://github.com/astral-sh/uv) is a fast Python package installer that significantly speeds up virtualenv creation:

```
Implement the API endpoints in src/api/.

Setup (using uv - much faster):
uv venv
source .venv/bin/activate
uv pip install -r requirements.txt

Then:
1. Create API handlers in src/api/handlers.py
2. Add routes in src/api/routes.py
3. Test: pytest tests/api/
```

### Shell Script Helper

Create a setup script for consistent initialization:

```bash
#!/bin/bash
# scripts/setup-worktree.sh

set -e

# Create virtualenv if it doesn't exist
if [ ! -d ".venv" ]; then
    python -m venv .venv
fi

# Activate
source .venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Install dev dependencies
pip install -r requirements-dev.txt

echo "Worktree ready!"
```

Reference in tasks:
```
Set up the worktree and implement the feature:

./scripts/setup-worktree.sh

Then implement the caching layer in src/cache/...
```

## Strategy 2: Poetry/PDM Projects

Best for: Projects using modern Python package managers.

### Poetry

Poetry manages virtualenvs automatically:

```
Implement the data processing module.

Setup:
poetry install

Then:
1. Create src/processing/pipeline.py
2. Add data transformations
3. Test: poetry run pytest tests/processing/
```

Configure Poetry to create virtualenvs in-project:

```bash
# Run once in main repo
poetry config virtualenvs.in-project true
```

### PDM

PDM also supports in-project virtualenvs:

```
Implement the notification service.

Setup:
pdm install

Then:
1. Create src/notifications/service.py
2. Add email and SMS handlers
3. Test: pdm run pytest tests/notifications/
```

## Strategy 3: Conda Environments

Best for: Data science and ML projects.

### Per-Worktree Conda Env

```
Implement the ML training pipeline.

Setup:
conda create -p ./.conda python=3.11 -y
conda activate ./.conda
pip install -r requirements.txt

Then:
1. Implement data preprocessing in src/ml/preprocess.py
2. Add model training in src/ml/train.py
3. Test: pytest tests/ml/
```

### Using environment.yml

```
Set up the data analysis environment.

Setup:
conda env create -f environment.yml -p ./.conda
conda activate ./.conda

Then implement the analysis pipeline...
```

## Framework-Specific Workflows

### Django

```
Add the user profile feature to the Django app.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Then:
1. Create profiles/models.py with UserProfile model
2. Create profiles/views.py with profile views
3. Add profiles/urls.py with URL patterns
4. Run migrations: python manage.py makemigrations && python manage.py migrate
5. Test: python manage.py test profiles
```

**Django-specific considerations:**
- Each worktree needs its own database (SQLite by default)
- Configure separate DATABASE_NAME per worktree for PostgreSQL
- Run migrations in each worktree independently

### Flask

```
Implement the API authentication endpoints.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Then:
1. Create app/auth/routes.py with login/logout/register
2. Add app/auth/decorators.py for auth decorators
3. Test: pytest tests/auth/
4. Run: FLASK_APP=app flask run --port 5001
```

### FastAPI

```
Add the items CRUD endpoints.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Then:
1. Create app/routers/items.py with CRUD operations
2. Add Pydantic schemas in app/schemas/items.py
3. Test: pytest tests/routers/test_items.py
4. Run: uvicorn app.main:app --port 8001 --reload
```

## Testing Strategies

### pytest (Parallel-Safe)

Each worktree runs pytest independently:

**Task 1:**
```
Run tests for the auth module:
source .venv/bin/activate
pytest tests/auth/ -v
```

**Task 2:**
```
Run tests for the api module:
source .venv/bin/activate
pytest tests/api/ -v
```

### pytest-xdist for Parallel Tests

Within a single worktree, speed up tests with parallel execution:

```
Run all tests in parallel:
source .venv/bin/activate
pip install pytest-xdist
pytest -n auto tests/
```

### Database Tests

For tests requiring a database:

**Option A: SQLite per Worktree**
```python
# conftest.py
import os
WORKTREE_ID = os.path.basename(os.getcwd())
DATABASE_URL = f"sqlite:///test_{WORKTREE_ID}.db"
```

**Option B: Test Containers**
```
Run integration tests with Docker:
source .venv/bin/activate
pytest tests/integration/ --docker
```

**Option C: Isolated Test Databases**
```bash
# Create test database per worktree
export TEST_DB_NAME="test_$(basename $PWD)"
pytest tests/
```

### Coverage Reports

Generate coverage per worktree:

```
Run tests with coverage:
source .venv/bin/activate
pytest --cov=src --cov-report=html tests/
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `requirements.txt` | HIGH | One instance handles deps |
| `pyproject.toml` | HIGH | Coordinate dependency changes |
| `poetry.lock` | HIGH | Same as pyproject.toml |
| `alembic/versions/` | MEDIUM | Sequential migrations |
| Source files | LOW | Different modules per instance |

### Task Design for Python

**Good decomposition:**
```
├── src/auth/     (authentication module)
├── src/api/      (API endpoints)
├── src/models/   (database models)
└── tests/        (corresponding tests)
```

**Risky decomposition:**
```
├── Add login feature (touches requirements, auth, api, models)
├── Add register feature (touches requirements, auth, api, models)
└── Add profile feature (touches requirements, api, models)
```

### Handling requirements.txt Conflicts

**Option A: Pre-install all dependencies**
```
First, update requirements.txt with all needed packages:
- Add pyjwt for authentication
- Add bcrypt for password hashing
- Add pydantic for validation

pip freeze > requirements.txt
```

**Option B: Separate requirement files**
```
requirements/
├── base.txt      # Core dependencies
├── auth.txt      # Auth-specific deps
├── api.txt       # API-specific deps
└── dev.txt       # Development deps
```

## Development Server Coordination

### Port Management

Avoid port conflicts with explicit port assignment:

**Task 1:**
```
Run the development server on port 8001:
source .venv/bin/activate
uvicorn app.main:app --port 8001
```

**Task 2:**
```
Run the development server on port 8002:
source .venv/bin/activate
uvicorn app.main:app --port 8002
```

### Environment Variables

Create `.env` files per worktree:

```
Set up environment and run:

1. Create .env with:
   DEBUG=true
   PORT=8001
   DATABASE_URL=sqlite:///db_abc123.sqlite

2. source .venv/bin/activate
3. python -m app.main
```

## Code Quality Tools

### Linting and Formatting

Include quality checks in tasks:

```
Implement the feature and ensure code quality:

source .venv/bin/activate
pip install -r requirements-dev.txt

1. Implement the feature in src/feature/
2. Format: black src/feature/
3. Lint: ruff check src/feature/
4. Type check: mypy src/feature/
5. Test: pytest tests/feature/
```

### Pre-commit Hooks

If using pre-commit, run it in each worktree:

```
Set up and run pre-commit:
source .venv/bin/activate
pre-commit install
pre-commit run --all-files
```

## Example: Building a Complete Feature

### Scenario

Implementing a task queue system with:
- Task model and database
- Queue service
- Worker implementation
- API endpoints

### Session Setup

```bash
claudio start task-queue-feature
```

### Tasks

**Task 1 - Models:**
```
Create task queue models in src/tasks/models.py.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implement:
1. Task model with status, payload, created_at, updated_at
2. TaskResult model for storing results
3. Alembic migration for new tables

Verify:
alembic upgrade head
pytest tests/tasks/test_models.py
```

**Task 2 - Queue Service:**
```
Create the queue service in src/tasks/queue.py.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implement:
1. TaskQueue class with enqueue, dequeue, get_status
2. Priority queue support
3. Retry logic for failed tasks

Test:
pytest tests/tasks/test_queue.py
```

**Task 3 - Worker:**
```
Create the task worker in src/tasks/worker.py.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implement:
1. Worker class that processes tasks
2. Graceful shutdown handling
3. Concurrent task processing
4. Error handling and retries

Test:
pytest tests/tasks/test_worker.py
```

**Task 4 - API Endpoints:**
```
Create API endpoints in src/api/tasks.py.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implement:
1. POST /tasks - create new task
2. GET /tasks/{id} - get task status
3. GET /tasks - list tasks with filters
4. DELETE /tasks/{id} - cancel task

Test:
pytest tests/api/test_tasks.py
```

## Performance Tips

### 1. Use uv Instead of pip

uv is 10-100x faster than pip:

```bash
# Install uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Use in tasks
uv venv
uv pip install -r requirements.txt
```

### 2. Pin Dependencies

Use pinned requirements for reproducible, faster installs:

```bash
# Generate pinned requirements
pip-compile requirements.in -o requirements.txt
```

### 3. Use pip's Cache

Ensure pip cache is enabled (default):

```bash
pip config get global.cache-dir
pip cache info
```

### 4. Minimal Virtual Environments

Include only needed dependencies:

```
# requirements-base.txt (core deps)
# requirements-dev.txt (dev tools)
# requirements-test.txt (test deps)

# For implementation tasks
pip install -r requirements-base.txt

# For test tasks
pip install -r requirements-base.txt -r requirements-test.txt
```

## Configuration Recommendations

For Python projects:

```yaml
# ~/.config/claudio/config.yaml

# Allow time for virtualenv setup
instance:
  activity_timeout_minutes: 45

# Python projects often have specific owners
pr:
  reviewers:
    by_path:
      "*.py": [python-team]
      "requirements*.txt": [tech-lead]
      "pyproject.toml": [tech-lead]
      "alembic/**": [backend-team, dba]

# ML projects can be expensive
resources:
  cost_warning_threshold: 15.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Python Tests

on:
  pull_request:
    paths:
      - '**/*.py'
      - 'requirements*.txt'
      - 'pyproject.toml'

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-python@v5
        with:
          python-version: '3.11'
          cache: 'pip'

      - name: Install dependencies
        run: |
          python -m pip install --upgrade pip
          pip install -r requirements.txt -r requirements-dev.txt

      - name: Lint
        run: |
          ruff check .
          black --check .

      - name: Type check
        run: mypy src/

      - name: Test
        run: pytest --cov=src --cov-report=xml tests/

      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

## Troubleshooting

### "ModuleNotFoundError: No module named 'X'"

Virtualenv not activated or package not installed.

**Solution**:
```bash
source .venv/bin/activate
pip install -r requirements.txt
```

### "virtualenv: command not found"

virtualenv not installed globally.

**Solution**:
```bash
pip install virtualenv
# Or use built-in venv
python -m venv .venv
```

### Permission denied on .venv

Virtualenv from another user or corrupted.

**Solution**:
```bash
rm -rf .venv
python -m venv .venv
```

### Import errors after branch changes

Virtualenv was created with different dependencies.

**Solution**:
```bash
pip install -r requirements.txt  # Update deps
# Or recreate virtualenv
rm -rf .venv && python -m venv .venv && pip install -r requirements.txt
```

### Alembic migration conflicts

Multiple instances created migrations.

**Solution**:
```bash
# Merge migrations manually or use autogenerate
alembic merge heads
# Or coordinate: one instance handles migrations
```

## What You Learned

- Managing virtual environments in worktrees
- Strategies for faster dependency installation
- Framework-specific considerations
- Testing approaches for Python projects
- Handling requirements.txt conflicts
- CI integration patterns

## Next Steps

- [Data Science Development](datascience-development.md) - ML and notebook workflows
- [Full-Stack Development](fullstack-development.md) - Python backends with frontends
- [Configuration Guide](../guide/configuration.md) - Customize for your team
