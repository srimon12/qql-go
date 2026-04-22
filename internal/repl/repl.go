package repl

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"github.com/srimon12/qql-go/internal/config"
	"github.com/srimon12/qql-go/internal/output"
)

const Prompt = "\033[32m\033[1mqql>\033[0m "

type QueryExecutor interface {
	Execute(query string) (string, error)
	Explain(query string) (string, error)
	ExecuteFile(path string, stopOnError bool) (string, error)
	DumpCollection(collection, outputPath string) (string, error)
}

type REPL struct {
	config    *config.Config
	executor  QueryExecutor
	outputter *output.Outputter
	reader    *bufio.Reader
	history   []string
	running   bool
}

func NewREPL(cfg *config.Config, executor QueryExecutor) *REPL {
	return &REPL{
		config:    cfg,
		executor:  executor,
		outputter: output.NewOutputter(),
		reader:    bufio.NewReader(os.Stdin),
		running:   true,
	}
}

func (r *REPL) Run() error {
	r.outputter.PrintBanner()
	r.outputter.Print(fmt.Sprintf("Connected to \033[36m%s\033[0m", r.config.URL))
	r.outputter.Print("Type \033[1mhelp\033[0m for available commands or \033[1mexit\033[0m to quit.\n")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		r.running = false
		fmt.Println()
	}()

	for r.running {
		fmt.Print(Prompt)
		line, err := r.readLine()
		if err != nil {
			if err == io.EOF {
				r.outputter.Print("\nBye.")
				break
			}
			r.outputter.PrintError(fmt.Sprintf("Read error: %v", err))
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		r.addToHistory(line)

		if err := r.handleCommand(line); err != nil {
			r.outputter.PrintError(err.Error())
		}
	}

	return nil
}

func (r *REPL) readLine() (string, error) {
	var input strings.Builder
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false
	escapeNext := false

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil && input.Len() == 0 {
			return "", err
		}
		if err != nil && input.Len() > 0 {
			input.WriteString(line)
			break
		}

		for i := 0; i < len(line); i++ {
			ch := rune(line[i])

			if escapeNext {
				escapeNext = false
				continue
			}

			if ch == '\\' && (inSingleQuote || inDoubleQuote) {
				escapeNext = true
				continue
			}

			if ch == '\'' && !inDoubleQuote {
				inSingleQuote = !inSingleQuote
				continue
			}

			if ch == '"' && !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
				continue
			}

			if !inSingleQuote && !inDoubleQuote {
				if ch == '(' || ch == '[' || ch == '{' {
					depth++
				} else if (ch == ')' || ch == ']' || ch == '}') && depth > 0 {
					depth--
				}
			}
		}

		input.WriteString(line)

		if depth == 0 && !inSingleQuote && !inDoubleQuote {
			break
		}

		fmt.Print("  -> ")
	}

	return input.String(), nil
}

func (r *REPL) handleCommand(cmd string) error {
	lower := strings.ToLower(cmd)

	if lower == "exit" || lower == "quit" || lower == "\\q" || lower == ":q" {
		r.outputter.Print("Bye.")
		r.running = false
		return nil
	}

	if lower == "help" || lower == "\\h" || lower == "?" {
		r.printHelp()
		return nil
	}

	if query, ok := cutCommandPrefix(cmd, "explain"); ok {
		plan, err := r.executor.Explain(query)
		if err != nil {
			return fmt.Errorf("explain error: %w", err)
		}
		r.outputter.PrintExplain(plan)
		return nil
	}

	if query, ok := cutCommandPrefix(cmd, "execute"); ok {
		result, err := r.executor.ExecuteFile(query, false)
		if err != nil {
			return fmt.Errorf("execute error: %w", err)
		}
		r.outputter.PrintSuccess(result)
		return nil
	}

	if query, ok := cutCommandPrefix(cmd, "\\e"); ok {
		result, err := r.executor.ExecuteFile(query, false)
		if err != nil {
			return fmt.Errorf("execute error: %w", err)
		}
		r.outputter.PrintSuccess(result)
		return nil
	}

	if dumpArgs, ok := cutCommandPrefix(cmd, "dump"); ok {
		parts := strings.Fields(dumpArgs)
		if len(parts) == 3 && strings.EqualFold(parts[0], "collection") {
			parts = parts[1:]
		}
		if len(parts) != 2 {
			return fmt.Errorf("dump error: usage DUMP [COLLECTION] <name> <output.qql>")
		}
		result, err := r.executor.DumpCollection(parts[0], parts[1])
		if err != nil {
			return fmt.Errorf("dump error: %w", err)
		}
		r.outputter.PrintSuccess(result)
		return nil
	}

	result, err := r.executor.Execute(cmd)
	if err != nil {
		return fmt.Errorf("execution error: %w", err)
	}

	r.outputter.PrintSuccess(result)
	return nil
}

func (r *REPL) printHelp() {
	help := `
\033[1mAvailable Statements:\033[0m

  \033[33mINSERT INTO\033[0m COLLECTION <name> \033[33mVALUES\033[0m {'text': '...', ...}
	  Insert a point. 'text' is required and auto-vectorized (cloud inference path).
      Optional: \033[33mUSING MODEL\033[0m '<model>'
	  Optional: \033[33mUSING HYBRID\033[0m [\033[33mDENSE MODEL\033[0m '<model>'] [\033[33mSPARSE MODEL\033[0m '<model>'] (cloud path)

  \033[33mCREATE COLLECTION\033[0m <name> [\033[33mHYBRID\033[0m]
	  Create a new collection. Add HYBRID for dense+sparse named vectors.

  \033[33mDROP COLLECTION\033[0m <name>
      Delete a collection and all its points.

  \033[33mSHOW COLLECTIONS\033[0m
      List all collections in the connected Qdrant instance.

  \033[33mSEARCH\033[0m <name> \033[33mSIMILAR TO\033[0m '<text>' \033[33mLIMIT\033[0m <n>
	  Semantic search by vector similarity (text embedding uses cloud inference path).
      Optional: \033[33mUSING MODEL\033[0m '<model>'
	  Optional: \033[33mUSING HYBRID\033[0m [\033[33mDENSE MODEL\033[0m '<model>'] [\033[33mSPARSE MODEL\033[0m '<model>'] (cloud path)
      Optional: \033[33mWHERE\033[0m <filter>
      Optional: \033[33mRERANK\033[0m [\033[33mMODEL\033[0m '<model>']
      Optional: \033[33mEXACT\033[0m
      Optional: \033[33mWITH\033[0m { hnsw_ef: <int>, exact: <bool>, acorn: <bool> }

  \033[33mDELETE FROM\033[0m <name> \033[33mWHERE id =\033[0m '<id>'
      Delete a point by its ID.

\033[1mBuilt-in Commands:\033[0m

  \033[36mhelp\033[0m, \033[36m?\033[0m           Show this help
  \033[36mexplain <query>\033[0m  Show query plan without executing
  \033[36mexecute <file>\033[0m  Run a .qql script file
  \033[36m\e <file>\033[0m        Shortcut for execute
  \033[36mdump <name> <file>\033[0m  Dump a collection to a .qql script file
  \033[36mexit\033[0m, \033[36mquit\033[0m      Exit the shell

\033[1mKeyboard Shortcuts:\033[0m

  Ctrl-C         Cancel current input
  Ctrl-D         Exit shell
`
	fmt.Print(help)
}

func (r *REPL) addToHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	for i, h := range r.history {
		if h == cmd {
			r.history = append(r.history[:i], r.history[i+1:]...)
			break
		}
	}

	r.history = append(r.history, cmd)
	if len(r.history) > 100 {
		r.history = r.history[len(r.history)-100:]
	}
}

func isWordChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-'
}

func cutCommandPrefix(input, prefix string) (string, bool) {
	if len(input) <= len(prefix) || !strings.EqualFold(input[:len(prefix)], prefix) {
		return "", false
	}

	next, _ := utf8.DecodeRuneInString(input[len(prefix):])
	if !unicode.IsSpace(next) {
		return "", false
	}

	return strings.TrimSpace(input[len(prefix):]), true
}
