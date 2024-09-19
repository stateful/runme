package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	base   = "https://home.runme.dev/"
	client = "Kernel"
)

type LookupEnv func(key string) (string, bool)

// Returns true if telemetry reporting is enabled, false otherwise.
func ReportUnlessNoTracking(logger *zap.Logger) bool {
	if v := os.Getenv("DO_NOT_TRACK"); v != "" && v != "0" && v != "false" {
		logger.Info("Telemetry reporting is disabled with DO_NOT_TRACK")
		return false
	}

	if v := os.Getenv("SCARF_NO_ANALYTICS"); v != "" && v != "0" && v != "false" {
		logger.Info("Telemetry reporting is disabled with SCARF_NO_ANALYTICS")
		return false
	}

	logger.Info("Telemetry reporting is enabled")

	go func() {
		err := report()
		if err != nil {
			logger.Warn("Error reporting telemetry", zap.Error(err))
		}
	}()

	return true
}

func report() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	encodedURL, err := buildURL(os.LookupEnv, client)
	if err != nil {
		return errors.Wrapf(err, "Error building telemtry URL")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", encodedURL.String(), nil)
	if err != nil {
		return errors.Wrapf(err, "Error creating telemetry request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, "Error sending telemetry request")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("error sending telemetry request: status_code=%d, status=%s", resp.StatusCode, resp.Status)
	}

	return nil
}

func buildURL(lookup LookupEnv, client string) (*url.URL, error) {
	baseAndClient := base + client

	props := []string{
		"extname",
		"extversion",
		"remotename",
		"appname",
		"product",
		"platform",
		"uikind",
	}

	params := url.Values{}
	for _, p := range props {
		addValue(lookup, &params, p)
	}

	// until we have a non-extension-bundled reporting strategy, lets error
	if len(params) == 0 {
		return nil, fmt.Errorf("no telemetry properties provided")
	}

	dst, err := url.Parse(baseAndClient)
	if err != nil {
		return nil, err
	}
	dst.RawQuery = params.Encode()

	return dst, nil
}

func addValue(lookup LookupEnv, params *url.Values, prop string) {
	if v, ok := lookup(fmt.Sprintf("TELEMETRY_%s", strings.ToUpper(prop))); ok {
		params.Add(strings.ToLower(prop), v)
	}
}
