package jupyter

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
)

var (
	jupyterMeter            = otel.Meter("github.com/runmedev/runme/v3/pkg/agent/jupyter")
	jupyterKernelStates     = mustMetric(jupyterMeter.Int64UpDownCounter("runme.jupyter.kernels", otelmetric.WithDescription("Current directly managed Jupyter kernels by lifecycle state."), otelmetric.WithUnit("{kernel}")))
	jupyterLifecycleLatency = mustMetric(jupyterMeter.Float64Histogram("runme.jupyter.lifecycle.duration", otelmetric.WithDescription("Duration of direct Jupyter kernel lifecycle operations."), otelmetric.WithUnit("s")))
	jupyterActiveBridges    = mustMetric(jupyterMeter.Int64UpDownCounter("runme.jupyter.bridges.active", otelmetric.WithDescription("Current direct browser-to-kernel Jupyter bridges."), otelmetric.WithUnit("{bridge}")))
	jupyterChannelMessages  = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.channel.messages", otelmetric.WithDescription("Direct Jupyter channel messages by direction and channel."), otelmetric.WithUnit("{message}")))
	jupyterChannelBytes     = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.channel.bytes", otelmetric.WithDescription("Direct Jupyter channel payload bytes by direction and channel."), otelmetric.WithUnit("By")))
	jupyterProtocolErrors   = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.protocol.errors", otelmetric.WithDescription("Rejected direct Jupyter protocol messages by bounded reason."), otelmetric.WithUnit("{error}")))
	jupyterRateLimits       = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.rate_limits", otelmetric.WithDescription("Direct Jupyter bridges closed by IOPub rate limits."), otelmetric.WithUnit("{event}")))
	jupyterHeartbeatMisses  = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.heartbeat.misses", otelmetric.WithDescription("Missed direct Jupyter kernel heartbeat probes."), otelmetric.WithUnit("{event}")))
	jupyterUnexpectedExits  = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.kernel.unexpected_exits", otelmetric.WithDescription("Unexpected exits of directly managed Jupyter kernels."), otelmetric.WithUnit("{event}")))
	jupyterBridgeCloses     = mustMetric(jupyterMeter.Int64Counter("runme.jupyter.bridge.closes", otelmetric.WithDescription("Direct Jupyter bridge closes by bounded reason."), otelmetric.WithUnit("{event}")))
)

func mustMetric[T any](instrument T, err error) T {
	if err != nil {
		panic(err)
	}
	return instrument
}

func observeLifecycle(operation string, started time.Time) {
	jupyterLifecycleLatency.Record(context.Background(), time.Since(started).Seconds(), otelmetric.WithAttributes(attribute.String("operation", operation)))
}

func addKernelState(state KernelState, delta int64) {
	jupyterKernelStates.Add(context.Background(), delta, otelmetric.WithAttributes(attribute.String("state", string(state))))
}

func addActiveBridge(delta int64) {
	jupyterActiveBridges.Add(context.Background(), delta)
}

func observeChannelMessage(direction string, channel Channel, payloadBytes int) {
	attributes := otelmetric.WithAttributes(
		attribute.String("direction", direction),
		attribute.String("channel", string(channel)),
	)
	jupyterChannelMessages.Add(context.Background(), 1, attributes)
	jupyterChannelBytes.Add(context.Background(), int64(payloadBytes), attributes)
}

func observeBridgeClose(reason string) {
	addCounter(jupyterBridgeCloses, attribute.String("reason", reason))
}

func addCounter(counter otelmetric.Int64Counter, attributes ...attribute.KeyValue) {
	counter.Add(context.Background(), 1, otelmetric.WithAttributes(attributes...))
}

func observeProtocolError(err error) {
	reason := "malformed_message"
	if errors.Is(err, ErrBinaryBuffersUnsupported) {
		reason = "unsupported_binary_buffers"
	} else if err != nil && strings.Contains(strings.ToLower(err.Error()), "signature") {
		reason = "bad_signature"
	}
	addCounter(jupyterProtocolErrors, attribute.String("reason", reason))
}
