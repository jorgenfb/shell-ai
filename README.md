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

1. Get a free API key from [Google AI Studio](https://aistudio.google.com/apikey) (no billing required)
2. Build and configure:

```bash
go build -o shell-ai .
mkdir -p ~/.config/shell-ai
echo 'api_key = "YOUR_GEMINI_API_KEY"' > ~/.config/shell-ai/config.toml
```

The default model (`gemini-flash-lite-latest`) works with free API keys. Optional config: `model = "gemini-flash-lite-latest"` (default).

## Examples

```bash
shell-ai find files larger than 100MB
shell-ai --explain how to compress a directory
shell-ai --yolo show current date
```
