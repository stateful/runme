package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	base   = "https://home.runme.dev/"
	client = "Kernel"
)

type (
	reporterFunc  func() error
	lookupEnvFunc func(key string) (string, bool)
)

var reporter reporterFunc

func init() {
	reporter = liveReporter
}

// Returns true if telemetry reporting is enabled, false otherwise.
func ReportUnlessNoTracking(logger *zap.Logger) bool {
	disablers := []string{"DO_NOT_TRACK", "SCARF_NO_ANALYTICS"}

	for _, key := range disablers {
		disabled, err := trackingDisabledForEnv(key)
		if err == nil && disabled {
			logger.Info(fmt.Sprintf("Telemetry reporting is disabled with %s", key))
			return false
		}
	}

	logger.Info("Telemetry reporting is enabled")

	go func() {
		err := reporter()
		if err != nil {
			logger.Warn("Error reporting telemetry", zap.Error(err))
		}
	}()

	return true
}

func trackingDisabledForEnv(key string) (bool, error) {
	val, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return false, err
	}

	return val, nil
}

func liveReporter() error {
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

func buildURL(lookup lookupEnvFunc, client string) (*url.URL, error) {
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

func addValue(lookup lookupEnvFunc, params *url.Values, prop string) {
	if v, ok := lookup(fmt.Sprintf("TELEMETRY_%s", strings.ToUpper(prop))); ok {
		params.Add(strings.ToLower(prop), v)
	}
}
