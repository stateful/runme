package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stateful/runme/internal/runner"
	"golang.org/x/oauth2/github"
)

const (
	apiURLF      = "api-url"
	authURLF     = "auth-url"
	traceF       = "trace"
	traceAllF    = "trace-all"
	enableChaosF = "enable-chaos"
)

var (
	apiBaseURL  string
	authBaseURL string
	trace       bool
	traceAll    bool
	enableChaos bool
)

func getAPIURL() string    { return apiBaseURL }
func getAuthURL() string   { return authBaseURL }
func getTrace() bool       { return trace || traceAll }
func getTraceAll() bool    { return traceAll }
func getEnableChaos() bool { return enableChaos }

// TODO(adamb): temporarily we authorize using Github as IdP.
// In the future, we will likely change this to Stateful being IdP.
// TODO(mxs): ditto
var defaultAuthURL = func() string {
	ghURL, err := url.Parse(github.Endpoint.AuthURL)
	if err != nil {
		panic(err)
	}
	return (&url.URL{Scheme: ghURL.Scheme, Host: ghURL.Host}).String()
}()

func setConfigFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVar(&authBaseURL, authURLF, defaultAuthURL, "backend URL to authorize you")
	flagSet.StringVar(&apiBaseURL, apiURLF, "https://api.stateful.com", "backend URL with API")
	flagSet.BoolVar(&trace, traceF, false, "trace HTTP calls")
	flagSet.BoolVar(&traceAll, traceAllF, false, "trace all HTTP calls including authentication (it might leak sensitive data to output)")
	flagSet.BoolVar(&enableChaos, enableChaosF, false, "enable Chaos Monkey mode for GraphQL requests")
}

func readMarkdownFile(args []string) ([]byte, error) {
	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	if arg == "" {
		f, err := os.DirFS(fChdir).Open(fFileName)
		if err != nil {
			var pathError *os.PathError
			if errors.As(err, &pathError) {
				return nil, errors.Errorf("failed to %s markdown file %s: %s", pathError.Op, pathError.Path, pathError.Err.Error())
			}

			return nil, errors.Wrapf(err, "failed to read %s", filepath.Join(fChdir, fFileName))
		}
		defer func() { _ = f.Close() }()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read data")
		}
		return data, nil
	}

	var (
		data []byte
		err  error
	)

	if arg == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read from stdin")
		}
	} else if strings.HasPrefix(arg, "https://") {
		client := http.Client{
			Timeout: time.Second * 5,
		}
		resp, err := client.Get(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get a file %q", arg)
		}
		defer func() { _ = resp.Body.Close() }()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read body")
		}
	} else {
		f, err := os.Open(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open file %q", arg)
		}
		defer func() { _ = f.Close() }()
		data, err = io.ReadAll(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from file %q", arg)
		}
	}

	return data, nil
}

func writeMarkdownFile(args []string, data []byte) error {
	arg := ""
	if len(args) == 1 {
		arg = args[0]
	}

	if arg == "-" {
		return errors.New("cannot write to stdin")
	}

	if strings.HasPrefix(arg, "https://") {
		return errors.New("cannot write to HTTP location")
	}

	fullFilename := arg
	if fullFilename == "" {
		fullFilename = filepath.Join(fChdir, fFileName)
	}
	err := os.WriteFile(fullFilename, data, 0)
	return errors.Wrapf(err, "failed to write to %s", fullFilename)
}

func getCodeBlocks() (document.CodeBlocks, error) {
	data, err := readMarkdownFile(nil)
	if err != nil {
		return nil, err
	}

	doc := document.New(data, cmark.Render)
	node, _, err := doc.Parse()
	if err != nil {
		return nil, err
	}

	blocks := document.CollectCodeBlocks(node)

	filtered := make(document.CodeBlocks, 0, len(blocks))
	for _, b := range blocks {
		if fAllowUnknown || (b.Language() != "" && runner.IsSupported(b.Language())) {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func lookupCodeBlock(blocks document.CodeBlocks, name string) (*document.CodeBlock, error) {
	block := blocks.Lookup(name)
	if block == nil {
		return nil, errors.Errorf("command %q not found; known command names: %s", name, blocks.Names())
	}
	return block, nil
}

func validCmdNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	blocks, err := getCodeBlocks()
	if err != nil {
		cmd.PrintErrf("failed to get parser: %s", err)
		return nil, cobra.ShellCompDirectiveError
	}

	names := blocks.Names()

	var filtered []string
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

func setDefaultFlags(cmd *cobra.Command) {
	usage := "Help for "
	if n := cmd.Name(); n != "" {
		usage += n
	} else {
		usage += "this command"
	}
	usage += "."
	cmd.Flags().BoolP("help", "h", false, usage)

	// For the root command, set up the --version flag.
	if cmd.Use == "runme" {
		usage := "Version of "
		if n := cmd.Name(); n != "" {
			usage += n
		} else {
			usage += "this command"
		}
		usage += "."
		cmd.Flags().BoolP("version", "v", false, usage)
	}
}

func printfInfo(msg string, args ...any) {
	var buf bytes.Buffer
	_, _ = buf.WriteString("\x1b[0;32m")
	_, _ = fmt.Fprintf(&buf, msg, args...)
	_, _ = buf.WriteString("\x1b[0m")
	_, _ = buf.WriteString("\r\n")
	_, _ = os.Stderr.Write(buf.Bytes())
}

func getDefaultConfigHome() string {
	// TODO(adamb): switch to os.UserConfigDir()
	// TODO(mxs): ditto
	dir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(dir, ".config", "stateful")
}

type runFunc func(context.Context) error
