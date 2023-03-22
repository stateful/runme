package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/document"
	"github.com/stateful/runme/internal/renderer/cmark"
	"github.com/stateful/runme/internal/runner"
	"go.uber.org/zap"
)

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
	cmd.Flags().BoolP("help", "h", false, usage)

	// For the root command, set up the --version flag.
	if cmd.Use == "runme" {
		usage := "Version of "
		if n := cmd.Name(); n != "" {
			usage += n
		} else {
			usage += "this command"
		}
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
	dir, err := os.UserHomeDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, ".config", "stateful")
}

type runFunc func(context.Context) error

func generateTLS(tlsDir string, logger *zap.Logger) (*tls.Config, error) {
	if info, err := os.Stat(tlsDir); err != nil {
		if err := os.MkdirAll(tlsDir, tlsFileMode); err != nil {
			return nil, err
		}
	} else {
		if !info.IsDir() {
			return nil, fmt.Errorf("provided tls path is not a directory: %s", tlsDir)
		}

		if err := os.Chmod(tlsDir, tlsFileMode); err != nil {
			return nil, err
		}
	}

	var (
		certPath = path.Join(tlsDir, "cert.pem")
		pkPath   = path.Join(tlsDir, "key.pem")
	)

	// TODO: rotation strategy here

	logger.Info("generating new TLS certificate...")

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "stateful",
			Organization: []string{"Stateful, INC."},
			Country:      []string{"US"},
			Province:     []string{"California"},
			Locality:     []string{"Berkeley"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 0, 30),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
		},
		DNSNames: []string{
			"localhost",
		},
	}

	certificateBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, err
	}

	caPEM := new(bytes.Buffer)
	if err := pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateBytes,
	}); err != nil {
		return nil, err
	}

	privKeyPEM := new(bytes.Buffer)
	if err := pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}); err != nil {
		return nil, err
	}

	// TODO: probably a more efficient way to create a `tls.Certificate`
	// rather than unencrypting the PEM again...
	tlsCa, err := tls.X509KeyPair(caPEM.Bytes(), privKeyPEM.Bytes())
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()

	// TODO: can probably use `AddCert` here
	if !certPool.AppendCertsFromPEM(caPEM.Bytes()) {
		return nil, fmt.Errorf("failed to add certificate to certificate pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCa},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}

	if err := os.WriteFile(certPath, caPEM.Bytes(), tlsFileMode); err != nil {
		return nil, err
	}

	if err := os.WriteFile(pkPath, privKeyPEM.Bytes(), tlsFileMode); err != nil {
		return nil, err
	}
	logger.Info("successfully generated new TLS certificate!")

	return tlsConfig, nil
}
