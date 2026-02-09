# Contributing to Steward

Thank you for your interest in contributing to Steward! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.

## Developer Certificate of Origin

By contributing to this project, you agree to the Developer Certificate of Origin (DCO). Every commit must be signed off:

```bash
git commit -s -m "Your commit message"
```

## Getting Started

### Prerequisites

- Go 1.24+
- kubectl
- A Kubernetes cluster for testing (KIND recommended)
- Helm (for installation testing)

### Setting Up Your Development Environment

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/steward.git
   cd steward
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/butlerdotdev/steward.git
   ```

### Building

```bash
make build
```

### Running Locally

```bash
make run
```

### Running Tests

```bash
make test
```

### Linting

```bash
make lint
```

## Making Changes

### Code Style

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Include Apache 2.0 license headers on all source files

### Commit Messages

Follow conventional commits:

```
type(scope): description

[optional body]

[optional footer]
Signed-off-by: Your Name <your.email@example.com>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `chore`: Maintenance tasks
- `refactor`: Code refactoring
- `test`: Adding or updating tests

### Pull Request Process

1. Create a feature branch:
   ```bash
   git checkout -b feat/your-feature
   ```

2. Make your changes and commit:
   ```bash
   git add .
   git commit -s -m "feat(controller): add new reconciliation logic"
   ```

3. Push to your fork:
   ```bash
   git push origin feat/your-feature
   ```

4. Open a Pull Request against `master`

5. Ensure all checks pass

## Feature Requests

For feature requests, please use the [Discussion's Feature Request section](https://github.com/butlerdotdev/steward/discussions/categories/feature-requests).

## Bug Reports

Check if the issue has been already opened in [GitHub Issues](https://github.com/butlerdotdev/steward/issues). If not, open a new one with a detailed report to help replicate, assess, and fix the issue.

## Getting Help

- Open an [issue](https://github.com/butlerdotdev/steward/issues) for bugs
- Start a [discussion](https://github.com/butlerdotdev/steward/discussions) for questions
- Check existing issues and discussions before creating new ones

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
