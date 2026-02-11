# shell-ai

CLI tool that uses Google Gemini to help with Linux shell commands. Ask a question in plain English, get back a command.

## Usage

```
shell-ai [--explain | --confirm | --yolo] <question...>
```

- `-e, --explain` — Show command with explanation
- `-c, --confirm` — Show command, ask before executing (default)
- `-y, --yolo` — Execute immediately

## Setup

```bash
go build -o shell-ai .
mkdir -p ~/.config/shell-ai
echo 'api_key = "YOUR_GEMINI_API_KEY"' > ~/.config/shell-ai/config.toml
```

Optional config: `model = "gemini-flash-lite-latest"` (default).

## Examples

```bash
shell-ai find files larger than 100MB
shell-ai --explain how to compress a directory
shell-ai --yolo show current date
```
