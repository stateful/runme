package jupyter

import (
	"errors"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	jupyterKernelStates     = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "runme_jupyter_kernels", Help: "Current directly managed Jupyter kernels by lifecycle state."}, []string{"state"})
	jupyterLifecycleLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "runme_jupyter_lifecycle_duration_seconds", Help: "Duration of direct Jupyter kernel lifecycle operations.", Buckets: prometheus.DefBuckets}, []string{"operation"})
	jupyterActiveBridges    = prometheus.NewGauge(prometheus.GaugeOpts{Name: "runme_jupyter_active_bridges", Help: "Current direct browser-to-kernel Jupyter bridges."})
	jupyterChannelMessages  = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runme_jupyter_channel_messages_total", Help: "Direct Jupyter channel messages by direction and channel."}, []string{"direction", "channel"})
	jupyterChannelBytes     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runme_jupyter_channel_bytes_total", Help: "Direct Jupyter channel payload bytes by direction and channel."}, []string{"direction", "channel"})
	jupyterProtocolErrors   = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runme_jupyter_protocol_errors_total", Help: "Rejected direct Jupyter protocol messages by bounded reason."}, []string{"reason"})
	jupyterRateLimits       = prometheus.NewCounter(prometheus.CounterOpts{Name: "runme_jupyter_rate_limits_total", Help: "Direct Jupyter bridges closed by IOPub rate limits."})
	jupyterHeartbeatMisses  = prometheus.NewCounter(prometheus.CounterOpts{Name: "runme_jupyter_heartbeat_misses_total", Help: "Missed direct Jupyter kernel heartbeat probes."})
	jupyterUnexpectedExits  = prometheus.NewCounter(prometheus.CounterOpts{Name: "runme_jupyter_unexpected_exits_total", Help: "Unexpected exits of directly managed Jupyter kernels."})
	jupyterBridgeCloses     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "runme_jupyter_bridge_closes_total", Help: "Direct Jupyter bridge closes by bounded reason."}, []string{"reason"})
)

func init() {
	prometheus.MustRegister(jupyterKernelStates, jupyterLifecycleLatency, jupyterActiveBridges, jupyterChannelMessages, jupyterChannelBytes, jupyterProtocolErrors, jupyterRateLimits, jupyterHeartbeatMisses, jupyterUnexpectedExits, jupyterBridgeCloses)
}

func observeLifecycle(operation string, started time.Time) {
	jupyterLifecycleLatency.WithLabelValues(operation).Observe(time.Since(started).Seconds())
}

func observeProtocolError(err error) {
	reason := "malformed_message"
	if errors.Is(err, ErrBinaryBuffersUnsupported) {
		reason = "unsupported_binary_buffers"
	} else if err != nil && strings.Contains(strings.ToLower(err.Error()), "signature") {
		reason = "bad_signature"
	}
	jupyterProtocolErrors.WithLabelValues(reason).Inc()
}
