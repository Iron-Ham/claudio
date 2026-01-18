# Data Science & ML Development: Using Claudio with Machine Learning Projects

**Time: 25-35 minutes**

This tutorial explains how to use Claudio effectively with data science and machine learning projects, covering Jupyter notebooks, experiment tracking, model development, and GPU resource management.

## Overview

Claudio's git worktree architecture provides unique benefits for ML projects:

- **Experiment isolation**: Each worktree can run different experiments
- **Model versioning**: Track model iterations across branches
- **Notebook management**: Work on multiple notebooks simultaneously
- **Data pipeline separation**: Develop data processing in parallel
- **Resource coordination**: Manage GPU allocation across experiments

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Python environment with ML libraries (PyTorch/TensorFlow/scikit-learn)
- Jupyter Lab/Notebook or VS Code with Jupyter extension
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding ML Projects and Git Worktrees

### Typical ML Project Structure

```
ml-project/
├── data/                   # Data directory (often gitignored)
├── notebooks/              # Jupyter notebooks
│   ├── exploration/        # EDA notebooks
│   ├── training/           # Training notebooks
│   └── evaluation/         # Evaluation notebooks
├── src/
│   ├── data/               # Data loading and processing
│   ├── models/             # Model architectures
│   ├── training/           # Training loops
│   └── utils/              # Utilities
├── experiments/            # Experiment configs
├── outputs/                # Model checkpoints, logs
├── tests/                  # Unit tests
├── requirements.txt
└── pyproject.toml
```

### Worktree Considerations

Each worktree needs:
- **Isolated virtual environment**: Different dependencies per experiment
- **Separate output directories**: Model checkpoints, logs
- **Experiment configuration**: Hyperparameters, data paths
- **Optionally shared data**: Large datasets can be symlinked

## Strategy 1: Experiment-Based Development (Recommended)

Best for: Comparing different model architectures or hyperparameters.

### Concept

Each instance runs a different experiment:

```
Instance 1: Baseline model training
Instance 2: Model with attention mechanism
Instance 3: Model with different optimizer
Instance 4: Data augmentation experiments
```

### Workflow

```bash
claudio start ml-experiments
```

**Task 1 - Baseline:**
```
Train baseline model without attention.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Experiment:
1. Create experiments/baseline.yaml with config
2. Train: python src/training/train.py --config experiments/baseline.yaml
3. Log metrics to experiments/baseline/metrics.json
4. Evaluate: python src/evaluation/evaluate.py --model outputs/baseline/model.pt
```

**Task 2 - Attention Model:**
```
Train model with attention mechanism.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Experiment:
1. Implement attention in src/models/attention.py
2. Create experiments/attention.yaml
3. Train: python src/training/train.py --config experiments/attention.yaml
4. Compare metrics with baseline
```

### Experiment Configuration

Structure experiments for parallel development:

```yaml
# experiments/baseline.yaml
model:
  name: baseline
  hidden_size: 256
  layers: 4

training:
  epochs: 100
  batch_size: 32
  learning_rate: 0.001

output:
  dir: outputs/${experiment_name}
  checkpoint_freq: 10
```

## Strategy 2: Pipeline-Based Development

Best for: Data pipeline and feature engineering work.

### Task Assignment

**Instance 1 - Data Loading:**
```
Implement data loading pipeline.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/data/dataset.py with PyTorch Dataset
2. Add data augmentation transforms
3. Write unit tests

pytest tests/data/
```

**Instance 2 - Feature Engineering:**
```
Implement feature engineering pipeline.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/data/features.py with feature extractors
2. Add feature normalization
3. Write validation tests

pytest tests/data/
```

**Instance 3 - Preprocessing:**
```
Implement data preprocessing.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/data/preprocess.py
2. Add cleaning and validation
3. Create preprocessing pipeline

pytest tests/data/
```

## Strategy 3: Model Architecture Exploration

Best for: Neural architecture search and model comparison.

### Parallel Architecture Development

**Instance 1 - CNN Architecture:**
```
Implement CNN model architecture.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/models/cnn.py
2. Add residual connections
3. Implement forward pass
4. Test with dummy data

pytest tests/models/
```

**Instance 2 - Transformer Architecture:**
```
Implement Transformer model.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/models/transformer.py
2. Implement multi-head attention
3. Add positional encoding
4. Test with dummy data

pytest tests/models/
```

## Jupyter Notebook Management

### Notebook Isolation

Each worktree can run Jupyter independently:

```
Start Jupyter in this worktree:

source .venv/bin/activate
jupyter lab --port 8889 --no-browser

# Different port per worktree:
# Worktree 1: port 8888
# Worktree 2: port 8889
# Worktree 3: port 8890
```

### Notebook Version Control

Use jupytext for cleaner diffs:

```
Set up jupytext for notebook versioning:

pip install jupytext
jupytext --set-formats ipynb,py:percent notebooks/*.ipynb

# Now .py files are tracked in git
# .ipynb can be gitignored or tracked
```

### Notebook Tasks

**Task 1 - EDA Notebook:**
```
Create data exploration notebook.

Setup:
source .venv/bin/activate
pip install -r requirements.txt

Create notebooks/exploration/data_eda.py:
1. Load and describe dataset
2. Visualize distributions
3. Identify missing values
4. Document findings

Convert to notebook:
jupytext notebooks/exploration/data_eda.py --to notebook
```

**Task 2 - Training Notebook:**
```
Create training experiment notebook.

Setup:
source .venv/bin/activate
pip install -r requirements.txt

Create notebooks/training/experiment_01.py:
1. Set up experiment tracking
2. Define training loop
3. Log metrics and artifacts
4. Save checkpoints

jupytext notebooks/training/experiment_01.py --to notebook
```

## GPU Resource Management

### Single GPU Coordination

When multiple worktrees share a GPU:

**Option A: Sequential Training**
```bash
claudio add "Train baseline model" --start
claudio add "Train attention model" --depends-on "baseline"
```

**Option B: GPU Memory Allocation**
```
Train with limited GPU memory:

CUDA_VISIBLE_DEVICES=0 python train.py --gpu-memory-fraction 0.45
```

**Option C: CPU for Development**
```
Develop and test on CPU, train on GPU later:

python train.py --device cpu --epochs 1 --debug
```

### Multi-GPU Setup

With multiple GPUs, assign per worktree:

**Worktree 1:**
```
Train on GPU 0:
CUDA_VISIBLE_DEVICES=0 python train.py
```

**Worktree 2:**
```
Train on GPU 1:
CUDA_VISIBLE_DEVICES=1 python train.py
```

### Remote GPU Resources

For cloud GPU usage:

```
Submit training job to cloud:

# AWS SageMaker
python scripts/submit_sagemaker.py --config experiments/baseline.yaml

# Google Cloud AI Platform
python scripts/submit_vertex.py --config experiments/baseline.yaml
```

## Experiment Tracking

### MLflow Integration

```
Set up MLflow experiment tracking:

Setup:
pip install mlflow

Usage in training:
import mlflow

mlflow.set_experiment("model-comparison")
with mlflow.start_run(run_name="baseline"):
    mlflow.log_params(config)
    # ... training loop
    mlflow.log_metric("accuracy", accuracy)
    mlflow.log_artifact("model.pt")
```

### Weights & Biases Integration

```
Set up W&B tracking:

Setup:
pip install wandb
wandb login

Usage:
import wandb
wandb.init(project="my-project", name="baseline-experiment")
wandb.config.update(config)
# ... training loop
wandb.log({"loss": loss, "accuracy": accuracy})
```

### DVC Integration

For data and model versioning:

```
Set up DVC for data versioning:

pip install dvc
dvc init
dvc add data/raw/dataset.csv
git add data/raw/dataset.csv.dvc .gitignore
```

## Data Management

### Large Dataset Handling

For large datasets, symlink to shared location:

```bash
# Create shared data directory
mkdir -p /data/shared/ml-project

# Symlink in each worktree
ln -s /data/shared/ml-project/data ./data
```

Task instruction:
```
This worktree uses shared data.

Data location: /data/shared/ml-project/data
Symlink created: ./data -> /data/shared/ml-project/data

Do not modify files in ./data directly.
Create processed versions in ./processed/
```

### Data Version Control

```
Track data versions with DVC:

dvc pull  # Get data for this experiment
dvc run -n preprocess -d data/raw -o data/processed python preprocess.py
```

## Testing Strategies

### Unit Tests

Test model components independently:

**Task 1:**
```
Test model forward pass:

pytest tests/models/test_forward.py -v
```

**Task 2:**
```
Test data pipeline:

pytest tests/data/test_pipeline.py -v
```

### Integration Tests

Test full training pipeline:

```
Run integration tests:

pytest tests/integration/ -v --slow
```

### Model Validation

Validate model outputs:

```
Run model validation:

python src/evaluation/validate.py --model outputs/model.pt
python src/evaluation/sanity_check.py --model outputs/model.pt
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `requirements.txt` | HIGH | Coordinate dependency changes |
| `pyproject.toml` | HIGH | One instance for config changes |
| Notebooks (`.ipynb`) | HIGH | Use jupytext, different notebooks |
| Model code | LOW | Different model files |
| Experiment configs | LOW | Different experiment names |

### Task Design for ML

**Good decomposition:**
```
├── src/data/          (Data team)
├── src/models/        (Model team)
├── src/training/      (Training team)
└── notebooks/         (Different notebooks)
```

**Risky decomposition:**
```
├── Full experiment 1  (touches all files)
├── Full experiment 2  (touches all files)
└── Full experiment 3  (touches all files)
```

## Environment Management

### Conda Environments

```
Create conda environment for this worktree:

conda create -p ./.conda python=3.11 -y
conda activate ./.conda
pip install -r requirements.txt
```

### Poetry

```
Set up Poetry environment:

poetry install
poetry run python train.py
```

### Docker

```
Build and run in Docker:

docker build -t ml-experiment:baseline .
docker run --gpus all -v $(pwd)/data:/app/data ml-experiment:baseline
```

## Example: Complete ML Feature

### Scenario

Implementing a new model with:
- Data preprocessing pipeline
- Model architecture
- Training loop
- Evaluation metrics

### Session Setup

```bash
claudio start new-model-feature
```

### Tasks

**Task 1 - Data Pipeline:**
```
Implement data preprocessing.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/data/preprocess.py with:
   - Data loading from CSV
   - Missing value handling
   - Feature normalization
   - Train/val/test split

2. Create src/data/dataset.py with:
   - PyTorch Dataset class
   - Data augmentation

Test:
pytest tests/data/ -v
```

**Task 2 - Model Architecture:**
```
Implement model architecture.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/models/new_model.py with:
   - Model class
   - Forward pass
   - Weight initialization

2. Create config in experiments/new_model.yaml

Test:
pytest tests/models/ -v
```

**Task 3 - Training Loop:**
```
Implement training pipeline.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/training/trainer.py with:
   - Training loop
   - Validation step
   - Checkpointing
   - Early stopping

2. Add experiment tracking

Test:
python src/training/train.py --config experiments/new_model.yaml --epochs 1 --debug
```

**Task 4 - Evaluation:**
```
Implement evaluation metrics.

Setup:
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

Implementation:
1. Create src/evaluation/metrics.py with:
   - Accuracy, precision, recall, F1
   - Confusion matrix
   - ROC/AUC

2. Create src/evaluation/evaluate.py

Test:
python src/evaluation/evaluate.py --model outputs/new_model/checkpoint.pt
```

## Configuration Recommendations

For ML projects:

```yaml
# ~/.config/claudio/config.yaml

# ML training can take a long time
instance:
  activity_timeout_minutes: 120
  completion_timeout_minutes: 240

# Assign reviewers by area
pr:
  reviewers:
    by_path:
      "src/data/**": [data-team]
      "src/models/**": [ml-team]
      "notebooks/**": [data-science-team]
      "experiments/**": [ml-team, tech-lead]
      "requirements*.txt": [tech-lead]

# ML development can be very expensive
resources:
  cost_warning_threshold: 25.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: ML CI

on:
  pull_request:
    paths:
      - '**/*.py'
      - 'requirements*.txt'
      - 'experiments/**'

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

      - name: Unit tests
        run: pytest tests/ -v --ignore=tests/integration

      - name: Model smoke test
        run: |
          python -c "from src.models import NewModel; m = NewModel(); print('Model loads OK')"

  integration:
    runs-on: ubuntu-latest
    needs: test
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-python@v5
        with:
          python-version: '3.11'
          cache: 'pip'

      - name: Install dependencies
        run: pip install -r requirements.txt

      - name: Training smoke test
        run: |
          python src/training/train.py \
            --config experiments/test.yaml \
            --epochs 1 \
            --batch-size 2 \
            --device cpu
```

## Troubleshooting

### CUDA out of memory

Training exhausting GPU memory.

**Solution**:
```python
# Reduce batch size
--batch-size 16

# Gradient accumulation
--accumulation-steps 4

# Mixed precision training
--fp16
```

### Package version conflicts

Different package versions needed.

**Solution**:
```bash
# Create fresh environment
rm -rf .venv
python -m venv .venv
pip install -r requirements.txt
```

### Notebook kernel not found

Kernel not matching environment.

**Solution**:
```bash
source .venv/bin/activate
python -m ipykernel install --user --name=ml-project
```

### Data not found

Symlink or path issues.

**Solution**:
```bash
# Check symlink
ls -la data/

# Recreate symlink
rm data
ln -s /path/to/shared/data ./data
```

### Experiment tracking conflicts

Multiple runs with same name.

**Solution**:
```python
# Use unique run names
import uuid
run_name = f"experiment_{uuid.uuid4().hex[:8]}"
```

## What You Learned

- Experiment isolation strategies
- Jupyter notebook management in worktrees
- GPU resource coordination
- Experiment tracking integration
- Data management for ML projects
- CI integration patterns

## Next Steps

- [Python Development](python-development.md) - Python-specific patterns
- [Full-Stack Development](fullstack-development.md) - ML with web services
- [Configuration Guide](../guide/configuration.md) - Customize for your team
