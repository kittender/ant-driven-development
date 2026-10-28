# ADD harness — notes for Claude

Start every new session by reading `docs/HANDOFF.md`: it holds the state of the project, the decisions taken and why, every result, the person's preferences and the next steps.

Short version:
- Ant-Driven Development (ADD): many small, short-lived agents ("ageants") coordinated by emoji signals and a deterministic Ecosystem. Concept 0.4 is in `docs/concept-0.4.md` (live original: https://claude.ai/artifact/7KBP9BCAqKypKDe9WpVsXq).
- This repository is the Go harness, Level 0 (signals and judgment of small local models). Standard library only. `go vet ./... && go test -race ./...`.
- Experiments: `go run ./cmd/add-eval ...` (see README). Decision rules are written in `docs/concept-0.4-outline.adoc` before each run and applied unchanged.
- The person writes JavaScript and is learning Go: explain with JS analogies, keep answers concise, English for code.
- Local models only (Ollama on a 16 GB Mac, no API key). Do not ask to install more models without a strong reason.
