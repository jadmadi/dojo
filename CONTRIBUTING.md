# Contributing to Dojo

Thank you for your interest in contributing to **Dojo**!

## How to Contribute

1. **Fork the Repository**: Create your personal fork on GitHub.
2. **Create a Feature Branch**: `git checkout -b feature/my-cool-feature`.
3. **Write Tests**: Ensure all new features have test coverage.
4. **Run the Quality Gate**:
   ```bash
   go test -race ./...
   go run . doctor --skills --security
   ```
5. **Open a Pull Request**: Submit your PR with a clear description of the problem solved.

## Code of Conduct

Dojo adopts the standard [Contributor Covenant (v2.1)](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) to foster an open, welcoming, diverse, inclusive, and healthy community.

## License

By contributing to Dojo, you agree that your contributions will be licensed under the [Apache License, Version 2.0](LICENSE).
