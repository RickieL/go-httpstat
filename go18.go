// +build go1.8

package httpstat

import (
	"context"
	"crypto/tls"
	"net/http/httptrace"
	"time"
)

// End sets the time when reading response is done.
// This must be called after reading response body.
func (r *Result) End(t time.Time) {
	r.trasferDone = t
	
	// This means there was no initial HTTP Request.
	// Skip setting value(ContentTransfer and Total will be zero).
	if r.dnsStart.IsZero() {
		return
	}

	r.ContentTransfer = r.trasferDone.Sub(r.transferStart)
	r.Total = r.trasferDone.Sub(r.dnsStart)
}

func withClientTrace(ctx context.Context, r *Result) context.Context {
	return httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		
		DNSStart: func(i httptrace.DNSStartInfo) {
			r.dnsStart = time.Now()
		},

		DNSDone: func(i httptrace.DNSDoneInfo) {
			r.dnsDone = time.Now()
			r.DNSLookup = r.dnsDone.Sub(r.dnsStart)
			r.NameLookup = r.DNSLookup
			
		},

		ConnectStart: func(_, _ string) {
			r.tcpStart = time.Now()
			// When connecting to IP (When no DNS lookup)
			if r.dnsStart.IsZero() {
				r.dnsStart = r.tcpStart
				r.dnsDone = r.tcpStart
			}
		},

		ConnectDone: func(network, addr string, err error) {
			r.tcpDone = time.Now()
			r.TCPConnection = r.tcpDone.Sub(r.tcpStart)
			r.Connect = r.tcpDone.Sub(r.dnsStart)
			
		},

		TLSHandshakeStart: func() {
			r.isTLS = true
			r.tlsStart = time.Now()
			
			if r.dnsStart.IsZero() && r.tcpStart.IsZero() {
				r.dnsStart = r.tlsStart
				r.dnsDone = r.tlsStart
				r.tcpStart =  r.tlsStart
				r.tcpDone =  r.tlsStart
			}
		},

		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			r.tlsDone = time.Now()
			r.TLSHandshake = r.tlsDone.Sub(r.tlsStart)
			r.Pretransfer = r.tlsDone.Sub(r.dnsStart)
			
		},

		GotConn: func(i httptrace.GotConnInfo) {
			// Handle when keepalive is used and connection is reused.
			// DNSStart(Done), ConnectStart(Done) and TLSHandshakeStart(Done) are skipped
			if i.Reused {
				r.isReused = true
				now := time.Now()
				r.dnsStart = now
				r.dnsDone = now
				r.tcpStart = now
				r.tcpDone = now
				if r.isTLS {
					r.tlsStart = now
					r.tlsDone = now
				}
			}
			
		},

		WroteRequest: func(info httptrace.WroteRequestInfo) {
			r.serverStart = time.Now()

			// When client doesn't use DialContext or using old (before go1.7) `net` pakcage, DNS/TCP hook is not called.
			if (r.dnsStart.IsZero() && r.tcpStart.IsZero())  {
				now := r.serverStart
				r.dnsStart = now
				r.dnsDone = now
				r.tcpStart = now
				r.tcpDone = now
			
			}
			
		},

		GotFirstResponseByte: func() {
			r.serverDone = time.Now()
			r.ServerProcessing = r.serverDone.Sub(r.serverStart)
			r.StartTransfer = r.serverDone.Sub(r.dnsStart)
			r.transferStart = r.serverDone
		},
	})
}
