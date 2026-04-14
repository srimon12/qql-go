package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Outputter struct {
	stdout io.Writer
	stderr io.Writer
}

func NewOutputter() *Outputter {
	return NewOutputterWithWriters(os.Stdout, os.Stderr)
}

func NewOutputterWithWriters(stdout, stderr io.Writer) *Outputter {
	return &Outputter{
		stdout: stdout,
		stderr: stderr,
	}
}

func (o *Outputter) Print(msg string) {
	fmt.Fprintln(o.stdout, msg)
}

func (o *Outputter) PrintSuccess(msg string) {
	fmt.Fprintf(o.stdout, "\033[32m✓\033[0m %s\n", msg)
}

func (o *Outputter) PrintError(msg string) {
	fmt.Fprintf(o.stderr, "\033[31m✗\033[0m %s\n", msg)
}

func (o *Outputter) PrintBanner() {
	banner := "\n\033[36m╔══════════════════════════════════════════╗\033[0m\n" +
		"\033[36m║\033[0m  \033[1;36mQQL — Qdrant Query Language\033[0m           \033[36m║\033[0m\n" +
		"\033[36m╚══════════════════════════════════════════╝\033[0m\n"
	fmt.Fprint(o.stdout, banner)
}

func (o *Outputter) PrintSection(title, content string) {
	fmt.Fprintf(o.stdout, "\033[1m%s\033[0m\n%s\n", title, content)
}

func (o *Outputter) PrintExplain(plan string) {
	o.PrintSection("Query Plan", plan)
}

func (o *Outputter) PrintJSON(value any, pretty bool) error {
	enc := json.NewEncoder(o.stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(value)
}

func (o *Outputter) PrintConnectionStatus(url string, healthy bool) {
	if healthy {
		o.PrintSuccess(fmt.Sprintf("Connected to %s", url))
		return
	}

	o.PrintError(fmt.Sprintf("Cannot connect to %s", url))
}
