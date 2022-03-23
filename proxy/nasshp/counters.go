package nasshp

import (
	"github.com/enfabrica/enkit/proxy/utils"
	"github.com/prometheus/client_golang/prometheus"
)

type AllowErrors struct {
	InvalidCookie     utils.Counter
	InvalidHostFormat utils.Counter
	InvalidHostName   utils.Counter
	Unauthorized      utils.Counter
}

type ProxyErrors struct {
	CookieInvalidParameters utils.Counter
	CookieInvalidAuth       utils.Counter

	ProxyInvalidAuth     utils.Counter
	ProxyInvalidPort     utils.Counter
	ProxyInvalidHost     utils.Counter
	ProxyCouldNotEncrypt utils.Counter
	ProxyAllow           AllowErrors

	ConnectInvalidSID utils.Counter
	ConnectInvalidAck utils.Counter
	ConnectInvalidPos utils.Counter
	ConnectAllow      AllowErrors

	SshFailedUpgrade utils.Counter
	SshResumeNoSID   utils.Counter
	SshCreateExists  utils.Counter
	SshDialFailed    utils.Counter
}

type ReadWriterCounters struct {
	BrowserWriterStarted utils.Counter
	BrowserWriterStopped utils.Counter
	BrowserWriterError   utils.Counter

	BrowserReaderStarted utils.Counter
	BrowserReaderStopped utils.Counter
	BrowserReaderError   utils.Counter
}

type ProxyCounters struct {
	ReadWriterCounters

	SshProxyStarted utils.Counter
	SshProxyStopped utils.Counter
}

type SessionCounters struct {
	Resumed utils.Counter
	Invalid utils.Counter
	Created utils.Counter

	Orphaned utils.Counter
	Deleted  utils.Counter
}

var (
	descPoolGets = prometheus.NewDesc(
		"nasshp_pool_gets",
		"Number of buffers retrieved from the pool",
		nil, nil,
	)
	descPoolPuts = prometheus.NewDesc(
		"nasshp_pool_puts",
		"Number of buffers returned to the pool",
		nil, nil,
	)
	descPoolNews = prometheus.NewDesc(
		"nasshp_pool_news",
		"Number of buffers created for the pool",
		nil, nil,
	)

	descSessionResumed = prometheus.NewDesc(
		"nasshp_sessions_resumed",
		"Number of times SIDs were found in the sessions table already",
		nil, nil,
	)

	descSessionInvalid = prometheus.NewDesc(
		"nasshp_sessions_invalid",
		"Number of times the state of a SID was found, but invalid - file a BUG!",
		nil, nil,
	)

	descSessionCreated = prometheus.NewDesc(
		"nasshp_sessions_created",
		"Number of times SIDs were not found in the session table, causing a new session to be created",
		nil, nil,
	)

	descSessionOrphaned = prometheus.NewDesc(
		"nasshp_sessions_orphaned",
		"Number of times SIDs were left in the session table for the browser to reconnect",
		nil, nil,
	)

	descSessionDeleted = prometheus.NewDesc(
		"nasshp_sessions_deleted",
		"Number of times SIDs were deleted from the session table as the connection terminated",
		nil, nil,
	)

	helpError = "Number of times the request to the url resulted in the specified error"

	descCookieInvalidParameters = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/cookie", "error": "invalid parameters", "type": "bad client"},
	)

	descCookieInvalidAuth = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/cookie", "error": "invalid auth", "type": "unauthorized"},
	)

	descProxyInvalidAuth = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid auth", "type": "unauthorized"},
	)

	descProxyInvalidPort = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid port", "type": "bad client"},
	)

	descProxyInvalidHost = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid host", "type": "bad client"},
	)

	descProxyCouldNotEncrypt = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "could not encrypt", "type": "internal"},
	)

	descProxyInvalidCookie = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid cookie", "type": "auth"},
	)

	descProxyInvalidHostFormat = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid host split", "type": "bad client"},
	)

	descProxyInvalidHostName = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "invalid host name", "type": "dns"},
	)

	descProxyUnauthorized = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/proxy", "error": "unauthorized user", "type": "auth"},
	)

	descConnectInvalidSID = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid sid", "type": "bad client"},
	)

	descConnectInvalidAck = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid ack", "type": "bad client"},
	)

	descConnectInvalidPos = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid pos", "type": "bad client"},
	)

	descConnectInvalidCookie = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid cookie", "type": "auth"},
	)

	descConnectInvalidHostFormat = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid host split", "type": "bad client"},
	)

	descConnectInvalidHostName = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "invalid host name", "type": "dns"},
	)

	descConnectUnauthorized = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "unauthorized user", "type": "auth"},
	)

	descSshFailedUpgrade = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "failed upgrade", "type": "bad client"},
	)

	descSshResumeNoSID = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "failed resume", "type": "bad client"},
	)

	descSshCreateExists = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "create existing", "type": "bad client"},
	)

	descSshDialFailed = prometheus.NewDesc(
		"nasshp_url_errors",
		helpError,
		nil, prometheus.Labels{"url": "/connect", "error": "dial failed", "type": "endpoint"},
	)

	helpBrowser = "Number of times a goroutine of the specified type was started/stopped/errored out"

	descBrowserWriterStarted = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "writer", "action": "started"},
	)

	descBrowserWriterStopped = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "writer", "action": "stopped"},
	)

	descBrowserWriterError = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "writer", "action": "error"},
	)

	descBrowserReaderStarted = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "reader", "action": "started"},
	)

	descBrowserReaderStopped = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "reader", "action": "stopped"},
	)

	descBrowserReaderError = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "reader", "action": "error"},
	)

	descSshProxyStarted = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "proxy", "action": "started"},
	)

	descSshProxyStopped = prometheus.NewDesc(
		"nasshp_browser",
		helpBrowser,
		nil, prometheus.Labels{"type": "proxy", "action": "stopped"},
	)
)

type nasshCollector NasshProxy

func (nc *nasshCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(nc, ch)
}

func (nc *nasshCollector) Collect(ch chan<- prometheus.Metric) {
	np := (*NasshProxy)(nc)

	gets, puts, news := np.pool.Stats()
	errors := &np.errors
	counters := &np.counters
	sessions := &np.sessions

	metrics := []struct {
		desc  *prometheus.Desc
		value uint64
	}{

		{descPoolGets, gets},
		{descPoolPuts, puts},
		{descPoolNews, news},

		{descCookieInvalidParameters, errors.CookieInvalidParameters.Get()},
		{descCookieInvalidAuth, errors.CookieInvalidAuth.Get()},
		{descProxyInvalidAuth, errors.ProxyInvalidAuth.Get()},
		{descProxyInvalidPort, errors.ProxyInvalidPort.Get()},
		{descProxyInvalidHost, errors.ProxyInvalidHost.Get()},
		{descProxyCouldNotEncrypt, errors.ProxyCouldNotEncrypt.Get()},

		{descProxyInvalidCookie, errors.ProxyAllow.InvalidCookie.Get()},
		{descProxyInvalidHostFormat, errors.ProxyAllow.InvalidHostFormat.Get()},
		{descProxyInvalidHostName, errors.ProxyAllow.InvalidHostName.Get()},
		{descProxyUnauthorized, errors.ProxyAllow.Unauthorized.Get()},

		{descConnectInvalidSID, errors.ConnectInvalidSID.Get()},
		{descConnectInvalidAck, errors.ConnectInvalidAck.Get()},
		{descConnectInvalidPos, errors.ConnectInvalidPos.Get()},

		{descConnectInvalidCookie, errors.ConnectAllow.InvalidCookie.Get()},
		{descConnectInvalidHostFormat, errors.ConnectAllow.InvalidHostFormat.Get()},
		{descConnectInvalidHostName, errors.ConnectAllow.InvalidHostName.Get()},
		{descConnectUnauthorized, errors.ConnectAllow.Unauthorized.Get()},

		{descSshFailedUpgrade, errors.SshFailedUpgrade.Get()},
		{descSshResumeNoSID, errors.SshResumeNoSID.Get()},
		{descSshCreateExists, errors.SshCreateExists.Get()},
		{descSshDialFailed, errors.SshDialFailed.Get()},

		{descBrowserWriterStarted, counters.BrowserWriterStarted.Get()},
		{descBrowserWriterStopped, counters.BrowserWriterStopped.Get()},
		{descBrowserWriterError, counters.BrowserWriterError.Get()},

		{descBrowserReaderStarted, counters.BrowserReaderStarted.Get()},
		{descBrowserReaderStopped, counters.BrowserReaderStopped.Get()},
		{descBrowserReaderError, counters.BrowserReaderError.Get()},

		{descSshProxyStarted, counters.SshProxyStarted.Get()},
		{descSshProxyStopped, counters.SshProxyStopped.Get()},

		{descSessionResumed, sessions.Resumed.Get()},
		{descSessionInvalid, sessions.Invalid.Get()},
		{descSessionCreated, sessions.Created.Get()},
		{descSessionOrphaned, sessions.Orphaned.Get()},
		{descSessionDeleted, sessions.Deleted.Get()},
	}

	for _, metric := range metrics {
		ch <- prometheus.MustNewConstMetric(metric.desc, prometheus.CounterValue, float64(metric.value))
	}
}
