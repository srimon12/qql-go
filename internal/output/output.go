package output

import (
	"encoding/json"
	"fmt"
	"os"
)

type Outputter struct {
	writer *os.File
}

func NewOutputter() *Outputter {
	return &Outputter{
		writer: os.Stdout,
	}
}

func (o *Outputter) Print(msg string) {
	fmt.Fprintln(o.writer, msg)
}

func (o *Outputter) PrintSuccess(msg string) {
	fmt.Fprintf(o.writer, "\033[32m✓\033[0m %s\n", msg)
}

func (o *Outputter) PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "\033[31m✗\033[0m %s\n", msg)
}

func (o *Outputter) PrintBanner() {
	banner := "\n\033[36m╔══════════════════════════════════════════╗\033[0m\n" +
		"\033[36m║\033[0m  \033[1;36mQQL — Qdrant Query Language\033[0m           \033[36m║\033[0m\n" +
		"\033[36m╚══════════════════════════════════════════╝\033[0m\n"
	fmt.Fprint(o.writer, banner)
}

func (o *Outputter) PrintSection(title, content string) {
	fmt.Fprintf(o.writer, "\033[1m%s\033[0m\n%s\n", title, content)
}

func (o *Outputter) PrintExplain(plan string) {
	o.PrintSection("Query Plan", plan)
}

func (o *Outputter) PrintJSON(value any, pretty bool) error {
	enc := json.NewEncoder(o.writer)
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
