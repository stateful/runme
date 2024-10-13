package command

import (
	"encoding/hex"
	"io"
)

type envCollectorFile struct {
	encKey   []byte
	encNonce []byte
	scanner  envScanner
	temp     *tempDirectory
}

var _ EnvCollector = (*envCollectorFile)(nil)

func newEnvCollectorFile(
	scanner envScanner,
	encKey []byte,
	encNonce []byte,
) (*envCollectorFile, error) {
	temp, err := newTempDirectory()
	if err != nil {
		return nil, err
	}

	return &envCollectorFile{
		encKey:   encKey,
		encNonce: encNonce,
		scanner:  scanner,
		temp:     temp,
	}, nil
}

func (c *envCollectorFile) Diff() (changed []string, deleted []string, _ error) {
	defer func() { _ = c.temp.Cleanup() }()

	initialReader, err := c.temp.Open(c.prePath())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = initialReader.Close() }()

	initial, err := c.scanner(initialReader)
	if err != nil {
		return nil, nil, err
	}

	finalReader, err := c.temp.Open(c.postPath())
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = finalReader.Close() }()

	final, err := c.scanner(finalReader)
	if err != nil {
		return nil, nil, err
	}

	return diffEnvs(initial, final)
}

func (c *envCollectorFile) ExtraEnv() []string {
	if c.encKey == nil || c.encNonce == nil {
		return nil
	}
	return []string{
		createEnv(envCollectorEncKeyEnvName, hex.EncodeToString(c.encKey)),
		createEnv(envCollectorEncNonceEnvName, hex.EncodeToString(c.encNonce)),
		createEnv(EnvCollectorSessionPrePathEnvName, c.prePath()),
		createEnv(EnvCollectorSessionPostPathEnvName, c.postPath()),
	}
}

func (c *envCollectorFile) SetOnShell(shell io.Writer) error {
	return setOnShell(shell, envDumpCommand, true, false, false, c.prePath(), c.postPath())
}

func (c *envCollectorFile) prePath() string {
	return c.temp.Join(".env_pre")
}

func (c *envCollectorFile) postPath() string {
	return c.temp.Join(".env_post")
}
