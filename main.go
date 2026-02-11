package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type config struct {
	APIKey string
	Model  string
}

type mode int

const (
	modeConfirm mode = iota
	modeExplain
	modeYolo
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiCyan  = "\033[36m"
)

// termRenderer buffers streamed text and renders complete lines with ANSI formatting.
type termRenderer struct {
	buf    strings.Builder
	inCode bool
}

func (r *termRenderer) write(text string) {
	r.buf.WriteString(text)
	r.flush(false)
}

func (r *termRenderer) close() {
	r.flush(true)
}

func (r *termRenderer) flush(final bool) {
	content := r.buf.String()
	for {
		idx := strings.Index(content, "\n")
		if idx == -1 {
			break
		}
		r.renderLine(content[:idx])
		fmt.Println()
		content = content[idx+1:]
	}
	r.buf.Reset()
	if final && content != "" {
		r.renderLine(content)
	} else {
		r.buf.WriteString(content)
	}
}

func (r *termRenderer) renderLine(line string) {
	trimmed := strings.TrimSpace(line)

	if strings.HasPrefix(trimmed, "```") {
		r.inCode = !r.inCode
		return
	}

	if r.inCode {
		fmt.Print("  " + ansiDim + line + ansiReset)
		return
	}

	for level := 3; level >= 1; level-- {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(trimmed, prefix) {
			fmt.Print(ansiBold + trimmed[len(prefix):] + ansiReset)
			return
		}
	}

	fmt.Print(renderInline(line))
}

func renderInline(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '*' && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "**")
			if end != -1 {
				out.WriteString(ansiBold)
				out.WriteString(renderInline(s[i+2 : i+2+end]))
				out.WriteString(ansiReset)
				i = i + 2 + end + 2
				continue
			}
		}
		if s[i] == '`' {
			end := strings.Index(s[i+1:], "`")
			if end != -1 {
				out.WriteString(ansiCyan)
				out.WriteString(s[i+1 : i+1+end])
				out.WriteString(ansiReset)
				i = i + 1 + end + 1
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func main() {
	m, question := parseArgs(os.Args[1:])
	if question == "" {
		fmt.Fprintln(os.Stderr, "Usage: shell-ai [--explain | --confirm | --yolo] <question...>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Flags:")
		fmt.Fprintln(os.Stderr, "  -e, --explain   Stream full explanation with command")
		fmt.Fprintln(os.Stderr, "  -c, --confirm   Show command, ask before executing (default)")
		fmt.Fprintln(os.Stderr, "  -y, --yolo      Execute the returned command immediately")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	systemPrompt := "You are a Linux shell expert. Return ONLY the shell command. No explanation, no markdown fences, no commentary."
	if m == modeExplain {
		systemPrompt = "You are a Linux shell expert. Provide the command and a clear explanation."
	}

	command, err := streamResponse(cfg, systemPrompt, question, m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch m {
	case modeExplain:
		fmt.Println()
	case modeConfirm:
		fmt.Println()
		if !confirm() {
			return
		}
		execute(command)
	case modeYolo:
		fmt.Println()
		execute(command)
	}
}

func parseArgs(args []string) (mode, string) {
	m := modeConfirm
	var rest []string

	for _, arg := range args {
		switch arg {
		case "--explain", "-e":
			m = modeExplain
		case "--confirm", "-c":
			m = modeConfirm
		case "--yolo", "-y":
			m = modeYolo
		default:
			rest = append(rest, arg)
		}
	}

	return m, strings.Join(rest, " ")
}

func loadConfig() (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, fmt.Errorf("cannot determine home directory: %w", err)
	}

	path := filepath.Join(home, ".config", "shell-ai", "config.toml")
	f, err := os.Open(path)
	if err != nil {
		return config{}, fmt.Errorf("cannot open config file: %s\n\nCreate it with:\n  mkdir -p ~/.config/shell-ai\n  echo 'api_key = \"YOUR_GEMINI_API_KEY\"' > ~/.config/shell-ai/config.toml", path)
	}
	defer f.Close()

	cfg := config{Model: "gemini-2.0-flash"}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, "\"'")

		switch key {
		case "api_key":
			cfg.APIKey = value
		case "model":
			cfg.Model = value
		}
	}

	if cfg.APIKey == "" {
		return config{}, fmt.Errorf("api_key not set in %s", path)
	}

	return cfg, nil
}

func streamResponse(cfg config, systemPrompt, question string, m mode) (string, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		cfg.Model, cfg.APIKey,
	)

	body := map[string]any{
		"system_instruction": map[string]any{
			"parts": []map[string]any{
				{"text": systemPrompt},
			},
		},
		"contents": []map[string]any{
			{
				"parts": []map[string]any{
					{"text": question},
				},
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(url, "application/json", strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(errBody))
	}

	var full strings.Builder
	var renderer *termRenderer
	if m == modeExplain {
		renderer = &termRenderer{}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		var chunk struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
			text := chunk.Candidates[0].Content.Parts[0].Text
			full.WriteString(text)
			if renderer != nil {
				renderer.write(text)
			} else {
				fmt.Print(text)
			}
		}
	}

	if renderer != nil {
		renderer.close()
	}

	return strings.TrimSpace(full.String()), nil
}

func confirm() bool {
	fmt.Print("Execute? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

func execute(command string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
