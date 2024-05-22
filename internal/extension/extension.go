package extension

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"

	"github.com/stateful/runme/v3/internal/log"
	"go.uber.org/zap"
)

const (
	DefaultExtensionName  = "stateful.runme"
	PlatformExtensionName = "stateful.platform"
)

//go:generate mockgen --build_flags=--mod=mod -destination=./extension_mock_gen.go -package=extension . Extensioner
type Extensioner interface {
	IsInstalled() (string, bool, error)
	Install() error
	InstallCommand() string
	Update() error
}

func New(isStateful bool) Extensioner {
	if isStateful {
		return newStateful()
	}

	return newDefault()
}

func newDefault() Extensioner {
	return &extensioner{
		extensionName:     DefaultExtensionName,
		allExtensionNames: []string{DefaultExtensionName},
	}
}

func newStateful() Extensioner {
	return &extensioner{
		extensionName:     PlatformExtensionName,
		allExtensionNames: []string{PlatformExtensionName},
	}
}

type extensioner struct {
	extensionName     string
	allExtensionNames []string
}

func (ext *extensioner) IsInstalled() (string, bool, error) {
	return IsInstalled(ext.allExtensionNames)
}
func (ext *extensioner) Install() error         { return Install(ext.extensionName) }
func (ext *extensioner) InstallCommand() string { return InstallCommand(ext.extensionName) }
func (ext *extensioner) Update() error          { return Update(ext.extensionName) }

func IsInstalled(allExtensionNames []string) (string, bool, error) {
	extensions, err := listExtensions()
	if err != nil {
		return "", false, err
	}
	ext, found, err := isInstalled(extensions, allExtensionNames)
	return ext.String(), found, err
}

func InstallCommand(extensionName string) string {
	return strings.Join(installCommand(extensionName, false), " ")
}

func Install(extensionName string) error {
	cmdSlice := installCommand(extensionName, false)
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	// TODO(adamb): error written to stderr is not returned
	return cmd.Run()
}

func Update(extensionName string) error {
	cmdSlice := installCommand(extensionName, true)
	cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	// TODO(adamb): error written to stderr is not returned
	return cmd.Run()
}

func isInstalled(extensions []ext, searchedNames []string) (ext, bool, error) {
	found := make(map[string]ext)
	for _, name := range searchedNames {
		found[name] = ext{}
	}

	for _, ext := range extensions {
		_, ok := found[ext.Name]
		if ok {
			found[ext.Name] = ext
		}
	}

	for _, name := range searchedNames {
		if found[name] != (ext{}) {
			return found[name], true, nil
		}
	}
	return ext{}, false, nil
}

func installCommand(extensionName string, force bool) []string {
	cmd := []string{"code", "--install-extension"}
	// --force will update if the extension is already installed.
	// If it is not installed, --force has no effect.
	if force {
		cmd = append(cmd, "--force")
	}
	return append(cmd, extensionName)
}

func isVSCodeInstalled() bool {
	return commandExists("code")
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	if err != nil {
		log.Get().Info("failed to detect program in PATH", zap.String("name", name), zap.Error(err))
	}
	return err == nil
}

type ext struct {
	Name    string
	Version string
}

func (e ext) String() string { return e.Name + "@" + e.Version }

func listExtensions() ([]ext, error) {
	if !isVSCodeInstalled() {
		return nil, errors.New(`command "code" is not available`)
	}

	buf := bytes.NewBuffer(nil)

	cmd := exec.Command("code", "--list-extensions", "--show-versions")
	cmd.Stdout = buf

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return parseExtensions(buf)
}

func parseExtensions(r io.Reader) (list []ext, _ error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore lines that do not contain the at sign.
		// This might happen when using Codespaces
		// which prints a header line.
		if line != "" && strings.Contains(line, "@") {
			nameAtVer := strings.Split(line, "@")
			list = append(list, ext{Name: nameAtVer[0], Version: nameAtVer[1]})
		}
	}
	return list, scanner.Err()
}
