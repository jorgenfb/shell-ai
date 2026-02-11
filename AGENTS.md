# Agent Guidelines

## Build

```bash
go build -ldflags="-s -w" -o shell-ai .
```

Always use `-ldflags="-s -w"` to strip debug info and symbol table for smaller binaries.

## Dependencies

No external dependencies. Use only the Go standard library.
