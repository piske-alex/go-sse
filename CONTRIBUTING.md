# Contributing to go-sse

Thank you for considering contributing to go-sse! This document provides guidelines and instructions for contributing to the project.

## Development Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Commit your changes: `git commit -am 'Add a feature'`
4. Push the branch: `git push origin feature/your-feature-name`
5. Submit a pull request

## Code Standards

- Follow idiomatic Go style (use `gofmt` or `goimports`)
- Write unit tests for new code
- Document public API with comments
- Keep functions small and focused
- Use meaningful variable and function names

## Running Tests

To run all tests:

```bash
go test ./...
```

To run tests with coverage:

```bash
go test -cover ./...
```

## Building

To build the project:

```bash
make build
```

Or manually:

```bash
go build -o bin/go-sse ./cmd/server
```

## Running Locally

To run the server locally:

```bash
make run
```

Or manually:

```bash
go run ./cmd/server/main.go
```

You can then access the example client at `examples/client.html` by opening it in a browser.

## Submitting Changes

1. Ensure your code follows the standards above
2. Write or update tests as necessary
3. Update documentation if needed
4. Squash related commits before submitting your PR
5. Explain what your changes do in the PR description

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.
