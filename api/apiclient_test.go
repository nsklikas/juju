// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package api_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juju/clock"
	"github.com/juju/clock/testclock"
	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/names/v5"
	proxyutils "github.com/juju/proxy"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/api/common"
	apitesting "github.com/juju/juju/api/testing"
	apiservererrors "github.com/juju/juju/apiserver/errors"
	apiservertesting "github.com/juju/juju/apiserver/testing"
	"github.com/juju/juju/controller"
	"github.com/juju/juju/core/network"
	jjtesting "github.com/juju/juju/juju/testing"
	"github.com/juju/juju/rpc"
	"github.com/juju/juju/rpc/jsoncodec"
	"github.com/juju/juju/rpc/params"
	jtesting "github.com/juju/juju/testing"
	"github.com/juju/juju/utils/proxy"
	jujuversion "github.com/juju/juju/version"
)

type apiclientSuite struct {
	jjtesting.JujuConnSuite
}

var _ = gc.Suite(&apiclientSuite{})

func (s *apiclientSuite) TestDialAPIToModel(c *gc.C) {
	info := s.APIInfo(c)
	conn, location, err := api.DialAPI(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer conn.Close()
	assertConnAddrForModel(c, location, info.Addrs[0], s.State.ModelUUID())
}

func (s *apiclientSuite) TestDialAPIToRoot(c *gc.C) {
	info := s.APIInfo(c)
	info.ModelTag = names.NewModelTag("")
	conn, location, err := api.DialAPI(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer conn.Close()
	assertConnAddrForRoot(c, location, info.Addrs[0])
}

func (s *apiclientSuite) TestDialAPIMultiple(c *gc.C) {
	// Create a socket that proxies to the API server.
	info := s.APIInfo(c)
	serverAddr := info.Addrs[0]
	proxy := testing.NewTCPProxy(c, serverAddr)
	defer proxy.Close()

	// Check that we can use the proxy to connect.
	info.Addrs = []string{proxy.Addr()}
	conn, location, err := api.DialAPI(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	conn.Close()
	assertConnAddrForModel(c, location, proxy.Addr(), s.State.ModelUUID())

	// Now break Addrs[0], and ensure that Addrs[1]
	// is successfully connected to.
	proxy.Close()

	info.Addrs = []string{proxy.Addr(), serverAddr}
	conn, location, err = api.DialAPI(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	conn.Close()
	assertConnAddrForModel(c, location, serverAddr, s.State.ModelUUID())
}

func (s *apiclientSuite) TestDialAPIWithProxy(c *gc.C) {
	info := s.APIInfo(c)
	opts := api.DialOpts{IPAddrResolver: apitesting.IPAddrResolverMap{
		"testing.invalid": {"0.1.1.1"},
	}}
	fakeAddr := "testing.invalid:1234"

	// Confirm that the proxy configuration is used. See:
	//     https://bugs.launchpad.net/juju/+bug/1698989
	//
	// TODO(axw) use github.com/elazarl/goproxy set up a real
	// forward proxy, and confirm that we can dial a successful
	// connection.
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" {
			http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Host != fakeAddr {
			http.Error(w, fmt.Sprintf("unexpected host %s", r.URL.Host), http.StatusBadRequest)
			return
		}
		http.Error(w, "🍵", http.StatusTeapot)
	}
	proxyServer := httptest.NewServer(http.HandlerFunc(handler))
	defer proxyServer.Close()

	err := proxy.DefaultConfig.Set(proxyutils.Settings{
		Https: proxyServer.Listener.Addr().String(),
	})
	c.Assert(err, jc.ErrorIsNil)
	defer proxy.DefaultConfig.Set(proxyutils.Settings{})

	// Check that we can use the proxy to connect.
	info.Addrs = []string{fakeAddr}
	_, _, err = api.DialAPI(info, opts)
	c.Assert(err, gc.ErrorMatches, "unable to connect to API: I'm a teapot")
}

func (s *apiclientSuite) TestDialAPIMultipleError(c *gc.C) {
	var addrs []string

	// count holds the number of times we've accepted a connection.
	var count int32
	for i := 0; i < 3; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		c.Assert(err, jc.ErrorIsNil)
		defer listener.Close()
		addrs = append(addrs, listener.Addr().String())
		go func() {
			for {
				client, err := listener.Accept()
				if err != nil {
					return
				}
				atomic.AddInt32(&count, 1)
				client.Close()
			}
		}()
	}
	info := s.APIInfo(c)
	info.Addrs = addrs
	_, _, err := api.DialAPI(info, api.DialOpts{})
	c.Assert(err, gc.ErrorMatches, `unable to connect to API: .*`)
	c.Assert(atomic.LoadInt32(&count), gc.Equals, int32(3))
}

func (s *apiclientSuite) TestVerifyCA(c *gc.C) {
	decodedCACert, _ := pem.Decode([]byte(jtesting.CACert))
	serverCertWithoutCA, _ := tls.X509KeyPair([]byte(jtesting.ServerCert), []byte(jtesting.ServerKey))
	serverCertWithSelfSignedCA, _ := tls.X509KeyPair([]byte(jtesting.ServerCert), []byte(jtesting.ServerKey))
	serverCertWithSelfSignedCA.Certificate = append(serverCertWithSelfSignedCA.Certificate, decodedCACert.Bytes)

	specs := []struct {
		descr        string
		serverCert   tls.Certificate
		verifyCA     func(host, endpoint string, caCert *x509.Certificate) error
		expConnCount int32
		errRegex     string
	}{
		{
			descr:      "VerifyCA provided but server does not present a CA cert",
			serverCert: serverCertWithoutCA,
			verifyCA: func(host, endpoint string, caCert *x509.Certificate) error {
				return errors.New("VerifyCA should not be called")
			},
			// Dial tries to fetch CAs, doesn't find any and
			// proceeds with the connection to the servers. This
			// would be the case where we connect to an older juju
			// controller.
			expConnCount: 2,
			errRegex:     `unable to connect to API: .*`,
		},
		{
			descr:      "no VerifyCA provided",
			serverCert: serverCertWithSelfSignedCA,
			// Dial connects to all servers
			expConnCount: 1,
			errRegex:     `unable to connect to API: .*`,
		},
		{
			descr:      "VerifyCA that always rejects certs",
			serverCert: serverCertWithSelfSignedCA,
			verifyCA: func(host, endpoint string, caCert *x509.Certificate) error {
				return errors.New("CA not trusted")
			},
			// Dial aborts after fetching CAs
			expConnCount: 1,
			errRegex:     "CA not trusted",
		},
		{
			descr:      "VerifyCA that always accepts certs",
			serverCert: serverCertWithSelfSignedCA,
			verifyCA: func(host, endpoint string, caCert *x509.Certificate) error {
				return nil
			},
			// Dial fetches CAs and then proceeds with the connection to the servers
			expConnCount: 2,
			errRegex:     `unable to connect to API: .*`,
		},
	}

	info := s.APIInfo(c)
	for specIndex, spec := range specs {
		c.Logf("test %d: %s", specIndex, spec.descr)

		// connCount holds the number of times we've accepted a connection.
		var connCount int32
		tlsConf := &tls.Config{
			Certificates: []tls.Certificate{spec.serverCert},
		}

		listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
		c.Assert(err, jc.ErrorIsNil)
		defer listener.Close()
		go func() {
			buf := make([]byte, 4)
			for {
				client, err := listener.Accept()
				if err != nil {
					return
				}
				atomic.AddInt32(&connCount, 1)

				// Do a dummy read to prevent the connection from
				// closing before the client can access the certs.
				_, _ = client.Read(buf)
				_ = client.Close()
			}
		}()

		atomic.StoreInt32(&connCount, 0)
		info.Addrs = []string{listener.Addr().String()}
		_, _, err = api.DialAPI(info, api.DialOpts{
			VerifyCA: spec.verifyCA,
		})
		c.Assert(err, gc.ErrorMatches, spec.errRegex)
		c.Assert(atomic.LoadInt32(&connCount), gc.Equals, spec.expConnCount)
	}
}

func (s *apiclientSuite) TestOpen(c *gc.C) {
	info := s.APIInfo(c)
	st, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer st.Close()

	c.Assert(st.Addr().String(), gc.Equals, "wss://"+info.Addrs[0])
	modelTag, ok := st.ModelTag()
	c.Assert(ok, jc.IsTrue)
	c.Assert(modelTag, gc.Equals, s.Model.ModelTag())

	remoteVersion, versionSet := st.ServerVersion()
	c.Assert(versionSet, jc.IsTrue)
	c.Assert(remoteVersion, gc.Equals, jujuversion.Current)

	c.Assert(api.CookieURL(st).String(), gc.Equals, "https://deadbeef-1bad-500d-9000-4b1d0d06f00d/")
}

func (s *apiclientSuite) TestOpenCookieURLUsesSNIHost(c *gc.C) {
	info := s.APIInfo(c)
	info.SNIHostName = "somehost"
	st, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer st.Close()

	c.Assert(api.CookieURL(st).String(), gc.Equals, "https://somehost/")
}

func (s *apiclientSuite) TestOpenCookieURLDefaultsToAddress(c *gc.C) {
	info := s.APIInfo(c)
	info.ControllerUUID = ""
	st, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer st.Close()

	c.Assert(api.CookieURL(st).String(), gc.Matches, "https://localhost:.*/")
}

func (s *apiclientSuite) TestOpenHonorsModelTag(c *gc.C) {
	info := s.APIInfo(c)

	// TODO(jam): 2014-06-05 http://pad.lv/1326802
	// we want to test this eventually, but for now s.APIInfo uses
	// conn.StateInfo() which doesn't know about ModelTag.
	// c.Check(info.ModelTag, gc.Equals, model.Tag())
	// c.Assert(info.ModelTag, gc.Not(gc.Equals), "")

	// We start by ensuring we have an invalid tag, and Open should fail.
	info.ModelTag = names.NewModelTag("0b501e7e-cafe-f00d-ba1d-b1a570c0e199")
	_, err := api.Open(info, api.DialOpts{})
	c.Assert(errors.Cause(err), gc.DeepEquals, &rpc.RequestError{
		Message: `unknown model: "0b501e7e-cafe-f00d-ba1d-b1a570c0e199"`,
		Code:    "model not found",
	})
	c.Check(params.ErrCode(err), gc.Equals, params.CodeModelNotFound)

	// Now set it to the right tag, and we should succeed.
	info.ModelTag = s.Model.ModelTag()
	st, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	st.Close()

	// Backwards compatibility, we should succeed if we do not set an
	// model tag
	info.ModelTag = names.NewModelTag("")
	st, err = api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	st.Close()
}

func (s *apiclientSuite) TestDialWebsocketStopsOtherDialAttempts(c *gc.C) {
	// Try to open the API with two addresses.
	// Wait for connection attempts to both.
	// Let one succeed.
	// Wait for the other to be canceled.

	type dialResponse struct {
		conn jsoncodec.JSONConn
	}
	type dialInfo struct {
		ctx      context.Context
		location string
		replyc   chan<- dialResponse
	}
	dialed := make(chan dialInfo)
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		reply := make(chan dialResponse)
		dialed <- dialInfo{
			ctx:      ctx,
			location: urlStr,
			replyc:   reply,
		}
		r := <-reply
		return r.conn, nil
	}
	conn0 := fakeConn{}
	clock := testclock.NewClock(time.Now())
	openDone := make(chan struct{})
	const dialAddressInterval = 50 * time.Millisecond
	go func() {
		defer close(openDone)
		conn, err := api.Open(&api.Info{
			Addrs: []string{
				"place1.example:1234",
				"place2.example:1234",
			},
			SkipLogin: true,
			CACert:    jtesting.CACert,
		}, api.DialOpts{
			Timeout:             5 * time.Second,
			RetryDelay:          1 * time.Second,
			DialAddressInterval: dialAddressInterval,
			DialWebsocket:       fakeDialer,
			Clock:               clock,
			IPAddrResolver: apitesting.IPAddrResolverMap{
				"place1.example": {"0.1.1.1"},
				"place2.example": {"0.2.2.2"},
			},
		})
		c.Check(api.UnderlyingConn(conn), gc.Equals, conn0)
		c.Check(err, jc.ErrorIsNil)
	}()

	place1 := "wss://place1.example:1234/api"
	place2 := "wss://place2.example:1234/api"
	// Wait for first connection, but don't
	// reply immediately because we want
	// to wait for the second connection before
	// letting the first one succeed.
	var info0 dialInfo
	select {
	case info0 = <-dialed:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for dial")
	}
	this := place1
	other := place2
	if info0.location != place1 {
		// We now randomly order what we will connect to. So we check
		// whether we first tried to connect to place1 or place2.
		// However, we should still be able to interrupt a second dial by
		// having the first one succeed.
		this = place2
		other = place1
	}

	c.Assert(info0.location, gc.Equals, this)

	var info1 dialInfo
	// Wait for the next dial to be made. Note that we wait for two
	// waiters because ContextWithTimeout as created by the
	// outer level of api.Open also waits.
	err := clock.WaitAdvance(dialAddressInterval, time.Second, 2)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case info1 = <-dialed:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for dial")
	}
	c.Assert(info1.location, gc.Equals, other)

	// Allow the first dial to succeed.
	info0.replyc <- dialResponse{
		conn: conn0,
	}

	// The Open returns immediately without waiting
	// for the second dial to complete.
	select {
	case <-openDone:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for connection")
	}

	// The second dial's context is canceled to tell
	// it to stop.
	select {
	case <-info1.ctx.Done():
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for context to be closed")
	}
	conn1 := fakeConn{
		closed: make(chan struct{}),
	}
	// Allow the second dial to succeed.
	info1.replyc <- dialResponse{
		conn: conn1,
	}
	// Check that the connection it returns is closed.
	select {
	case <-conn1.closed:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for connection to be closed")
	}
}

type apiDialInfo struct {
	location   string
	hasRootCAs bool
	serverName string
}

var openWithSNIHostnameTests = []struct {
	about      string
	info       *api.Info
	expectDial apiDialInfo
}{{
	about: "no cert; DNS name - use SNI hostname",
	info: &api.Info{
		Addrs:       []string{"foo.com:1234"},
		SNIHostName: "foo.com",
		SkipLogin:   true,
	},
	expectDial: apiDialInfo{
		location:   "wss://foo.com:1234/api",
		hasRootCAs: false,
		serverName: "foo.com",
	},
}, {
	about: "no cert; numeric IP address - use SNI hostname",
	info: &api.Info{
		Addrs:       []string{"0.1.2.3:1234"},
		SNIHostName: "foo.com",
		SkipLogin:   true,
	},
	expectDial: apiDialInfo{
		location:   "wss://0.1.2.3:1234/api",
		hasRootCAs: false,
		serverName: "foo.com",
	},
}, {
	about: "with cert; DNS name - use cert",
	info: &api.Info{
		Addrs:       []string{"0.1.1.1:1234"},
		SNIHostName: "foo.com",
		SkipLogin:   true,
		CACert:      jtesting.CACert,
	},
	expectDial: apiDialInfo{
		location:   "wss://0.1.1.1:1234/api",
		hasRootCAs: true,
		serverName: "juju-apiserver",
	},
}, {
	about: "with cert; numeric IP address - use cert",
	info: &api.Info{
		Addrs:       []string{"0.1.2.3:1234"},
		SNIHostName: "foo.com",
		SkipLogin:   true,
		CACert:      jtesting.CACert,
	},
	expectDial: apiDialInfo{
		location:   "wss://0.1.2.3:1234/api",
		hasRootCAs: true,
		serverName: "juju-apiserver",
	},
}}

func (s *apiclientSuite) TestOpenWithSNIHostname(c *gc.C) {
	for i, test := range openWithSNIHostnameTests {
		c.Logf("test %d: %v", i, test.about)
		s.testOpenDialError(c, dialTest{
			apiInfo:         test.info,
			expectOpenError: `unable to connect to API: nope`,
			expectDials: []dialAttempt{{
				check: func(info dialInfo) {
					c.Check(info.location, gc.Equals, test.expectDial.location)
					c.Assert(info.tlsConfig, gc.NotNil)
					c.Check(info.tlsConfig.RootCAs != nil, gc.Equals, test.expectDial.hasRootCAs)
					c.Check(info.tlsConfig.ServerName, gc.Equals, test.expectDial.serverName)
				},
				returnError: errors.New("nope"),
			}},
			allowMoreDials: true,
		})
	}
}

func (s *apiclientSuite) TestFallbackToSNIHostnameOnCertErrorAndNonNumericHostname(c *gc.C) {
	s.testOpenDialError(c, dialTest{
		apiInfo: &api.Info{
			Addrs:       []string{"x.com:1234"},
			CACert:      jtesting.CACert,
			SNIHostName: "foo.com",
		},
		// go 1.9 says "is not authorized to sign for this name"
		// go 1.10 says "is not authorized to sign for this domain"
		expectOpenError: `unable to connect to API: x509: a root or intermediate certificate is not authorized to sign.*`,
		expectDials: []dialAttempt{{
			// The first dial attempt should use the private CA cert.
			check: func(info dialInfo) {
				c.Assert(info.tlsConfig, gc.NotNil)
				c.Check(info.tlsConfig.RootCAs.Subjects(), gc.HasLen, 1)
				c.Check(info.tlsConfig.ServerName, gc.Equals, "juju-apiserver")
			},
			returnError: x509.CertificateInvalidError{
				Reason: x509.CANotAuthorizedForThisName,
			},
		}, {
			// The second dial attempt should fall back to using the
			// SNI hostname.
			check: func(info dialInfo) {
				c.Assert(info.tlsConfig, gc.NotNil)
				c.Check(info.tlsConfig.RootCAs, gc.IsNil)
				c.Check(info.tlsConfig.ServerName, gc.Equals, "foo.com")
			},
			// Note: we return another certificate error so that
			// the Open logic returns immediately rather than waiting
			// for the timeout.
			returnError: x509.SystemRootsError{},
		}},
	})
}

func (s *apiclientSuite) TestFailImmediatelyOnCertErrorAndNumericHostname(c *gc.C) {
	s.testOpenDialError(c, dialTest{
		apiInfo: &api.Info{
			Addrs:  []string{"0.1.2.3:1234"},
			CACert: jtesting.CACert,
		},
		// go 1.9 says "is not authorized to sign for this name"
		// go 1.10 says "is not authorized to sign for this domain"
		expectOpenError: `unable to connect to API: x509: a root or intermediate certificate is not authorized to sign.*`,
		expectDials: []dialAttempt{{
			// The first dial attempt should use the private CA cert.
			check: func(info dialInfo) {
				c.Assert(info.tlsConfig, gc.NotNil)
				c.Check(info.tlsConfig.RootCAs.Subjects(), gc.HasLen, 1)
				c.Check(info.tlsConfig.ServerName, gc.Equals, "juju-apiserver")
			},
			returnError: x509.CertificateInvalidError{
				Reason: x509.CANotAuthorizedForThisName,
			},
		}},
	})
}

type dialTest struct {
	apiInfo *api.Info
	// expectDials holds an entry for each dial
	// attempt that's expected to be made.
	// If allowMoreDials is true, any number of
	// attempts will be allowed and the last entry
	// of expectDials will be used when the
	// number exceeds
	expectDials     []dialAttempt
	allowMoreDials  bool
	expectOpenError string
}

type dialAttempt struct {
	check       func(info dialInfo)
	returnError error
}

type dialInfo struct {
	location  string
	tlsConfig *tls.Config
	errc      chan<- error
}

func (s *apiclientSuite) testOpenDialError(c *gc.C, t dialTest) {
	dialed := make(chan dialInfo)
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		reply := make(chan error)
		dialed <- dialInfo{
			location:  urlStr,
			tlsConfig: tlsConfig,
			errc:      reply,
		}
		return nil, <-reply
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := api.Open(t.apiInfo, api.DialOpts{
			DialWebsocket:  fakeDialer,
			IPAddrResolver: seqResolver(t.apiInfo.Addrs...),
			Clock:          &fakeClock{},
		})
		c.Check(conn, gc.Equals, nil)
		c.Check(err, gc.ErrorMatches, t.expectOpenError)
	}()
	for i := 0; t.allowMoreDials || i < len(t.expectDials); i++ {
		c.Logf("attempt %d", i)
		var attempt dialAttempt
		if i < len(t.expectDials) {
			attempt = t.expectDials[i]
		} else if t.allowMoreDials {
			attempt = t.expectDials[len(t.expectDials)-1]
		} else {
			break
		}
		select {
		case info := <-dialed:
			attempt.check(info)
			info.errc <- attempt.returnError
		case <-done:
			if i < len(t.expectDials) {
				c.Fatalf("Open returned early - expected dials not made")
			}
			return
		case <-time.After(jtesting.LongWait):
			c.Fatalf("timed out waiting for dial")
		}
	}
	select {
	case <-done:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for API open")
	}
}

func (s *apiclientSuite) TestOpenWithNoCACert(c *gc.C) {
	// This is hard to test as we have no way of affecting the system roots,
	// so instead we check that the error that we get implies that
	// we're using the system roots.

	info := s.APIInfo(c)
	info.CACert = ""

	// Unfortunately I have not better way to check that there is no retry.
	// The idea is that if we don't have any retry, we should have a total dial time lesser than
	// the retryDelay. It may break if the dial doesn't fail fast enough, but 200ms is quite long
	// for this test, so it shouldn't be flaky.
	dialTime := time.Now()
	retryDelay := 200 * time.Millisecond

	// This test used to use a long timeout so that we can check that the retry
	// logic doesn't retry, but that got all messed up with dualstack IPs.
	// The api server was only listening on IPv4, but localhost resolved to both
	// IPv4 and IPv6. The IPv4 didn't retry, but the IPv6 one did, because it was
	// retrying the dial. The parallel try doesn't have a fatal error type yet.
	_, err := api.Open(info, api.DialOpts{
		Timeout:    10 * retryDelay,
		RetryDelay: retryDelay,
	})
	endDialTime := time.Now()

	c.Assert(err, gc.ErrorMatches, `unable to connect to API:.*x509: certificate signed by unknown authority`)
	c.Assert(endDialTime.Sub(dialTime), jc.DurationLessThan, retryDelay)

}

func (s *apiclientSuite) TestOpenWithRedirect(c *gc.C) {
	redirectToHosts := []string{"0.1.2.3:1234", "0.1.2.4:1235"}
	redirectToCACert := "fake CA cert"

	srv := apiservertesting.NewAPIServer(func(modelUUID string) interface{} {
		return &redirectAPI{
			modelUUID:        modelUUID,
			redirectToHosts:  redirectToHosts,
			redirectToCACert: redirectToCACert,
		}
	})
	defer srv.Close()

	_, err := api.Open(&api.Info{
		Addrs:    srv.Addrs,
		CACert:   jtesting.CACert,
		ModelTag: names.NewModelTag("beef1beef1-0000-0000-000011112222"),
	}, api.DialOpts{})
	c.Assert(err, gc.ErrorMatches, `redirection to alternative server required`)

	hps := make(network.MachineHostPorts, len(redirectToHosts))
	for i, addr := range redirectToHosts {
		hp, err := network.ParseMachineHostPort(addr)
		c.Assert(err, jc.ErrorIsNil)
		hps[i] = *hp
	}

	c.Assert(errors.Cause(err), jc.DeepEquals, &api.RedirectError{
		Servers:        []network.MachineHostPorts{hps},
		CACert:         redirectToCACert,
		FollowRedirect: true,
	})
}

func (s *apiclientSuite) TestOpenCachesDNS(c *gc.C) {
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		return fakeConn{}, nil
	}
	dnsCache := make(dnsCacheMap)
	conn, err := api.Open(&api.Info{
		Addrs: []string{
			"place1.example:1234",
		},
		SkipLogin: true,
		CACert:    jtesting.CACert,
	}, api.DialOpts{
		DialWebsocket: fakeDialer,
		IPAddrResolver: apitesting.IPAddrResolverMap{
			"place1.example": {"0.1.1.1"},
		},
		DNSCache: dnsCache,
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(conn, gc.NotNil)
	c.Assert(dnsCache.Lookup("place1.example"), jc.DeepEquals, []string{"0.1.1.1"})
}

// We want open to perform a DNS lookup against the host without the segments,
// but for the opening of the connect maintain the segments i.e.,
// jimm.com/my-segment/api
func (s *apiclientSuite) TestOpenCachesDNSAndRemovesSegments(c *gc.C) {
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		return fakeConn{}, nil
	}
	dnsCache := make(dnsCacheMap)

	conn, err := api.Open(
		&api.Info{
			Addrs: []string{
				"place1.example:1234/segment",
			},
			SkipLogin: true,
			CACert:    jtesting.CACert,
		},
		api.DialOpts{
			DialWebsocket: fakeDialer,
			IPAddrResolver: apitesting.IPAddrResolverMap{
				"place1.example": {"0.1.1.1"},
			},
			DNSCache: dnsCache,
		},
	)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(conn, gc.NotNil)

	c.Assert(dnsCache.Lookup("place1.example"), jc.DeepEquals, []string{"0.1.1.1"})
}

func (s *apiclientSuite) TestDNSCacheUsed(c *gc.C) {
	var dialed string
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		dialed = ipAddr
		return fakeConn{}, nil
	}
	conn, err := api.Open(&api.Info{
		Addrs: []string{
			"place1.example:1234",
		},
		SkipLogin: true,
		CACert:    jtesting.CACert,
	}, api.DialOpts{
		DialWebsocket: fakeDialer,
		// Note: don't resolve any addresses. If we resolve one,
		// then there's a possibility that the resolving will
		// happen and a second dial attempt will happen before
		// the Open returns, giving rise to a race.
		IPAddrResolver: apitesting.IPAddrResolverMap{},
		DNSCache: dnsCacheMap{
			"place1.example": {"0.1.1.1"},
		},
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(conn, gc.NotNil)
	// The dialed IP address should have come from the cache, not the IP address
	// resolver.
	c.Assert(dialed, gc.Equals, "0.1.1.1:1234")
	c.Assert(conn.IPAddr(), gc.Equals, "0.1.1.1:1234")
}

func (s *apiclientSuite) TestNumericAddressIsNotAddedToCache(c *gc.C) {
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		return fakeConn{}, nil
	}
	dnsCache := make(dnsCacheMap)
	conn, err := api.Open(&api.Info{
		Addrs: []string{
			"0.1.2.3:1234",
		},
		SkipLogin: true,
		CACert:    jtesting.CACert,
	}, api.DialOpts{
		DialWebsocket:  fakeDialer,
		IPAddrResolver: apitesting.IPAddrResolverMap{},
		DNSCache:       dnsCache,
	})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(conn, gc.NotNil)
	c.Assert(conn.Addr().String(), gc.Equals, "wss://0.1.2.3:1234")
	c.Assert(conn.IPAddr(), gc.Equals, "0.1.2.3:1234")
	c.Assert(dnsCache, gc.HasLen, 0)
}

func (s *apiclientSuite) TestFallbackToIPLookupWhenCacheOutOfDate(c *gc.C) {
	dialc := make(chan string)
	start := make(chan struct{})
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		dialc <- ipAddr
		<-start
		if ipAddr == "0.2.2.2:1234" {
			return fakeConn{}, nil
		}
		return nil, errors.Errorf("bad address")
	}
	dnsCache := dnsCacheMap{
		"place1.example": {"0.1.1.1"},
	}
	type openResult struct {
		conn api.Connection
		err  error
	}
	openc := make(chan openResult)
	go func() {
		conn, err := api.Open(&api.Info{
			Addrs: []string{
				"place1.example:1234",
			},
			SkipLogin: true,
			CACert:    jtesting.CACert,
		}, api.DialOpts{
			// Note: zero timeout means each address attempt
			// will only try once only.
			DialWebsocket: fakeDialer,
			IPAddrResolver: apitesting.IPAddrResolverMap{
				"place1.example": {"0.2.2.2"},
			},
			DNSCache: dnsCache,
		})
		openc <- openResult{conn, err}
	}()
	// Wait for both dial attempts to happen.
	// If we don't, then the second attempt might
	// happen before the first one and the first
	// attempt might then never happen.
	dialed := make(map[string]bool)
	for i := 0; i < 2; i++ {
		select {
		case hostPort := <-dialc:
			dialed[hostPort] = true
		case <-time.After(jtesting.LongWait):
			c.Fatalf("timed out waiting for dial attempt")
		}
	}
	// Allow the dial attempts to return.
	close(start)
	// Check that no more dial attempts happen.
	select {
	case hostPort := <-dialc:
		c.Fatalf("unexpected dial attempt to %q; existing attempts: %v", hostPort, dialed)
	case <-time.After(jtesting.ShortWait):
	}
	r := <-openc
	c.Assert(r.err, jc.ErrorIsNil)
	c.Assert(r.conn, gc.NotNil)
	c.Assert(r.conn.Addr().String(), gc.Equals, "wss://place1.example:1234")
	c.Assert(r.conn.IPAddr(), gc.Equals, "0.2.2.2:1234")
	c.Assert(dialed, jc.DeepEquals, map[string]bool{
		"0.2.2.2:1234": true,
		"0.1.1.1:1234": true,
	})
	c.Assert(dnsCache.Lookup("place1.example"), jc.DeepEquals, []string{"0.2.2.2"})
}

func (s *apiclientSuite) TestOpenTimesOutOnLogin(c *gc.C) {
	unblock := make(chan chan struct{})
	srv := apiservertesting.NewAPIServer(func(modelUUID string) interface{} {
		return &loginTimeoutAPI{
			unblock: unblock,
		}
	})
	defer srv.Close()
	defer close(unblock)

	clk := testclock.NewClock(time.Now())
	done := make(chan error, 1)
	go func() {
		_, err := api.Open(&api.Info{
			Addrs:    srv.Addrs,
			CACert:   jtesting.CACert,
			ModelTag: names.NewModelTag("beef1beef1-0000-0000-000011112222"),
		}, api.DialOpts{
			Clock:   clk,
			Timeout: 5 * time.Second,
		})
		done <- err
	}()
	// Wait for Login to be entered before we advance the clock. Note that we don't actually unblock the request,
	// we just ensure that the other side has gotten to the point where it wants to be blocked. Otherwise we might
	// advance the clock before we even get the api.Dial to finish or before TLS handshaking finishes.
	unblocked := make(chan struct{})
	defer close(unblocked)
	select {
	case unblock <- unblocked:
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for Login to be called")
	}
	err := clk.WaitAdvance(5*time.Second, time.Second, 1)
	c.Assert(err, jc.ErrorIsNil)
	select {
	case err := <-done:
		c.Assert(err, gc.ErrorMatches, `cannot log in: context deadline exceeded`)
	case <-time.After(time.Second):
		c.Fatalf("timed out waiting for api.Open timeout")
	}
}

func (s *apiclientSuite) TestOpenTimeoutAffectsDial(c *gc.C) {
	sync := make(chan struct{})
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		close(sync)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	clk := testclock.NewClock(time.Now())
	done := make(chan error, 1)
	go func() {
		_, err := api.Open(&api.Info{
			Addrs:     []string{"127.0.0.1:1234"},
			CACert:    jtesting.CACert,
			ModelTag:  names.NewModelTag("beef1beef1-0000-0000-000011112222"),
			SkipLogin: true,
		}, api.DialOpts{
			Clock:         clk,
			Timeout:       5 * time.Second,
			DialWebsocket: fakeDialer,
		})
		done <- err
	}()
	// Before we advance time, ensure that the parallel try mechanism
	// has entered the dial function.
	select {
	case <-sync:
	case <-time.After(testing.LongWait):
		c.Errorf("didn't enter dial")
	}
	err := clk.WaitAdvance(5*time.Second, time.Second, 1)
	c.Assert(err, jc.ErrorIsNil)
	select {
	case err := <-done:
		c.Assert(err, gc.ErrorMatches, `unable to connect to API: context deadline exceeded`)
	case <-time.After(time.Second):
		c.Fatalf("timed out waiting for api.Open timeout")
	}
}

func (s *apiclientSuite) TestOpenDialTimeoutAffectsDial(c *gc.C) {
	sync := make(chan struct{})
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		close(sync)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	clk := testclock.NewClock(time.Now())
	done := make(chan error, 1)
	go func() {
		_, err := api.Open(&api.Info{
			Addrs:     []string{"127.0.0.1:1234"},
			CACert:    jtesting.CACert,
			ModelTag:  names.NewModelTag("beef1beef1-0000-0000-000011112222"),
			SkipLogin: true,
		}, api.DialOpts{
			Clock:         clk,
			Timeout:       5 * time.Second,
			DialTimeout:   3 * time.Second,
			DialWebsocket: fakeDialer,
		})
		done <- err
	}()
	// Before we advance time, ensure that the parallel try mechanism
	// has entered the dial function.
	select {
	case <-sync:
	case <-time.After(testing.LongWait):
		c.Errorf("didn't enter dial")
	}
	err := clk.WaitAdvance(3*time.Second, time.Second, 2) // Timeout & DialTimeout
	c.Assert(err, jc.ErrorIsNil)
	select {
	case err := <-done:
		c.Assert(err, gc.ErrorMatches, `unable to connect to API: context deadline exceeded`)
	case <-time.After(time.Second):
		c.Fatalf("timed out waiting for api.Open timeout")
	}
}

func (s *apiclientSuite) TestOpenDialTimeoutDoesNotAffectLogin(c *gc.C) {
	unblock := make(chan chan struct{})
	srv := apiservertesting.NewAPIServer(func(modelUUID string) interface{} {
		return &loginTimeoutAPI{
			unblock: unblock,
		}
	})
	defer srv.Close()
	defer close(unblock)

	clk := testclock.NewClock(time.Now())
	done := make(chan error, 1)
	go func() {
		_, err := api.Open(&api.Info{
			Addrs:    srv.Addrs,
			CACert:   jtesting.CACert,
			ModelTag: names.NewModelTag("beef1beef1-0000-0000-000011112222"),
		}, api.DialOpts{
			Clock:       clk,
			DialTimeout: 5 * time.Second,
		})
		done <- err
	}()

	// We should not get a response from api.Open until we
	// unblock the login.
	unblocked := make(chan struct{})
	select {
	case unblock <- unblocked:
		// We are now in the Login method of the loginTimeoutAPI.
	case <-time.After(jtesting.LongWait):
		c.Fatalf("didn't enter Login")
	}

	// There should be nothing waiting. Advance the clock to where it
	// would have triggered the DialTimeout. But this doesn't stop api.Open
	// as we have already connected and entered Login.
	err := clk.WaitAdvance(5*time.Second, 0, 0)
	c.Assert(err, jc.ErrorIsNil)

	// Ensure that api.Open doesn't return until we tell it to.
	select {
	case <-done:
		c.Fatalf("unexpected return from api.Open")
	case <-time.After(jtesting.ShortWait):
	}

	// unblock the login by sending to "unblocked", and then the
	// api.Open should return the result of the login.
	close(unblocked)
	select {
	case err := <-done:
		c.Assert(err, gc.ErrorMatches, "login failed")
	case <-time.After(jtesting.LongWait):
		c.Fatalf("timed out waiting for api.Open to return")
	}
}

func (s *apiclientSuite) TestWithUnresolvableAddr(c *gc.C) {
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		c.Errorf("dial was called but should not have been")
		return nil, errors.Errorf("cannot dial")
	}
	conn, err := api.Open(&api.Info{
		Addrs: []string{
			"nowhere.example:1234",
		},
		SkipLogin: true,
		CACert:    jtesting.CACert,
	}, api.DialOpts{
		DialWebsocket:  fakeDialer,
		IPAddrResolver: apitesting.IPAddrResolverMap{},
	})
	c.Assert(err, gc.ErrorMatches, `cannot resolve "nowhere.example": mock resolver cannot resolve "nowhere.example"`)
	c.Assert(conn, jc.ErrorIsNil)
}

func (s *apiclientSuite) TestWithUnresolvableAddrAfterCacheFallback(c *gc.C) {
	var dialedReal bool
	fakeDialer := func(ctx context.Context, urlStr string, tlsConfig *tls.Config, ipAddr string) (jsoncodec.JSONConn, error) {
		if ipAddr == "0.2.2.2:1234" {
			dialedReal = true
			return nil, errors.Errorf("cannot connect with real address")
		}
		return nil, errors.Errorf("bad address from cache")
	}
	dnsCache := dnsCacheMap{
		"place1.example": {"0.1.1.1"},
	}
	conn, err := api.Open(&api.Info{
		Addrs: []string{
			"place1.example:1234",
		},
		SkipLogin: true,
		CACert:    jtesting.CACert,
	}, api.DialOpts{
		DialWebsocket: fakeDialer,
		IPAddrResolver: apitesting.IPAddrResolverMap{
			"place1.example": {"0.2.2.2"},
		},
		DNSCache: dnsCache,
	})
	c.Assert(err, gc.NotNil)
	c.Assert(conn, gc.Equals, nil)
	c.Assert(dnsCache.Lookup("place1.example"), jc.DeepEquals, []string{"0.2.2.2"})
	c.Assert(dialedReal, jc.IsTrue)
}

func (s *apiclientSuite) TestAPICallNoError(c *gc.C) {
	clock := &fakeClock{}
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(),
		Clock:         clock,
	})

	err := conn.APICall("facade", 1, "id", "method", nil, nil)
	c.Check(err, jc.ErrorIsNil)
	c.Check(clock.waits, gc.HasLen, 0)
}

func (s *apiclientSuite) TestAPICallErrorBadRequest(c *gc.C) {
	clock := &fakeClock{}
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(errors.BadRequestf("boom")),
		Clock:         clock,
	})

	err := conn.APICall("facade", 1, "id", "method", nil, nil)
	c.Check(err.Error(), gc.Equals, "boom")
	c.Check(err, jc.ErrorIs, errors.BadRequest)
	c.Check(clock.waits, gc.HasLen, 0)
}

func (s *apiclientSuite) TestAPICallErrorNotImplemented(c *gc.C) {
	clock := &fakeClock{}
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(apiservererrors.ServerError(errors.NotImplementedf("boom"))),
		Clock:         clock,
	})

	err := conn.APICall("facade", 1, "id", "method", nil, nil)
	c.Check(err, jc.ErrorIs, errors.NotImplemented)
	c.Check(clock.waits, gc.HasLen, 0)
}

func (s *apiclientSuite) TestIsBrokenOk(c *gc.C) {
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(),
		Clock:         new(fakeClock),
	})
	c.Assert(conn.IsBroken(), jc.IsFalse)
}

func (s *apiclientSuite) TestIsBrokenChannelClosed(c *gc.C) {
	broken := make(chan struct{})
	close(broken)
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(),
		Clock:         new(fakeClock),
		Broken:        broken,
	})
	c.Assert(conn.IsBroken(), jc.IsTrue)
}

func (s *apiclientSuite) TestIsBrokenPingFailed(c *gc.C) {
	conn := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: newRPCConnection(errors.New("no biscuit")),
		Clock:         new(fakeClock),
	})
	c.Assert(conn.IsBroken(), jc.IsTrue)
}

func (s *apiclientSuite) TestLoginCapturesCLIArgs(c *gc.C) {
	s.PatchValue(&os.Args, []string{"this", "is", "the test", "command"})

	info := s.APIInfo(c)
	conn := newRPCConnection()
	conn.response = &params.LoginResult{
		ControllerTag: "controller-" + s.ControllerConfig.ControllerUUID(),
		ServerVersion: "2.3-rc2",
	}
	// Pass an already-closed channel so we don't wait for the monitor
	// to signal the rpc connection is dead when closing the state
	// (because there's no monitor running).
	broken := make(chan struct{})
	close(broken)
	testState := api.NewTestingState(c, api.TestingStateParams{
		RPCConnection: conn,
		Clock:         &fakeClock{},
		Address:       "wss://localhost:1234",
		Broken:        broken,
		Closed:        make(chan struct{}),
	})
	err := testState.Login(info.Tag, info.Password, "", nil)
	c.Assert(err, jc.ErrorIsNil)

	calls := conn.stub.Calls()
	c.Assert(calls, gc.HasLen, 1)
	call := calls[0]
	c.Assert(call.FuncName, gc.Equals, "Admin.Login")
	c.Assert(call.Args, gc.HasLen, 2)
	request := call.Args[1].(*params.LoginRequest)
	c.Assert(request.CLIArgs, gc.Equals, `this is "the test" command`)
}

func (s *apiclientSuite) TestConnectStreamRequiresSlashPathPrefix(c *gc.C) {
	reader, err := s.APIState.ConnectStream("foo", nil)
	c.Assert(err, gc.ErrorMatches, `cannot make API path from non-slash-prefixed path "foo"`)
	c.Assert(reader, gc.Equals, nil)
}

func (s *apiclientSuite) TestConnectStreamErrorBadConnection(c *gc.C) {
	s.PatchValue(&api.WebsocketDial, func(_ api.WebsocketDialer, _ string, _ http.Header) (base.Stream, error) {
		return nil, fmt.Errorf("bad connection")
	})
	reader, err := s.APIState.ConnectStream("/", nil)
	c.Assert(err, gc.ErrorMatches, "bad connection")
	c.Assert(reader, gc.IsNil)
}

func (s *apiclientSuite) TestConnectStreamErrorNoData(c *gc.C) {
	s.PatchValue(&api.WebsocketDial, func(_ api.WebsocketDialer, _ string, _ http.Header) (base.Stream, error) {
		return api.NewFakeStreamReader(&bytes.Buffer{}), nil
	})
	reader, err := s.APIState.ConnectStream("/", nil)
	c.Assert(err, gc.ErrorMatches, "unable to read initial response: EOF")
	c.Assert(reader, gc.IsNil)
}

func (s *apiclientSuite) TestConnectStreamErrorBadData(c *gc.C) {
	s.PatchValue(&api.WebsocketDial, func(_ api.WebsocketDialer, _ string, _ http.Header) (base.Stream, error) {
		return api.NewFakeStreamReader(strings.NewReader("junk\n")), nil
	})
	reader, err := s.APIState.ConnectStream("/", nil)
	c.Assert(err, gc.ErrorMatches, "unable to unmarshal initial response: .*")
	c.Assert(reader, gc.IsNil)
}

func (s *apiclientSuite) TestConnectStreamErrorReadError(c *gc.C) {
	s.PatchValue(&api.WebsocketDial, func(_ api.WebsocketDialer, _ string, _ http.Header) (base.Stream, error) {
		err := fmt.Errorf("bad read")
		return api.NewFakeStreamReader(&badReader{err}), nil
	})
	reader, err := s.APIState.ConnectStream("/", nil)
	c.Assert(err, gc.ErrorMatches, "unable to read initial response: bad read")
	c.Assert(reader, gc.IsNil)
}

// badReader raises err when Read is called.
type badReader struct {
	err error
}

func (r *badReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

func (s *apiclientSuite) TestConnectControllerStreamRejectsRelativePaths(c *gc.C) {
	reader, err := s.APIState.ConnectControllerStream("foo", nil, nil)
	c.Assert(err, gc.ErrorMatches, `path "foo" is not absolute`)
	c.Assert(reader, gc.IsNil)
}

func (s *apiclientSuite) TestConnectControllerStreamRejectsModelPaths(c *gc.C) {
	reader, err := s.APIState.ConnectControllerStream("/model/foo", nil, nil)
	c.Assert(err, gc.ErrorMatches, `path "/model/foo" is model-specific`)
	c.Assert(reader, gc.IsNil)
}

func (s *apiclientSuite) TestConnectControllerStreamAppliesHeaders(c *gc.C) {
	catcher := api.UrlCatcher{}
	headers := http.Header{}
	headers.Add("thomas", "cromwell")
	headers.Add("anne", "boleyn")
	s.PatchValue(&api.WebsocketDial, catcher.RecordLocation)

	_, err := s.APIState.ConnectControllerStream("/something", nil, headers)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(catcher.Headers().Get("thomas"), gc.Equals, "cromwell")
	c.Assert(catcher.Headers().Get("anne"), gc.Equals, "boleyn")
}

func (s *apiclientSuite) TestWatchDebugLogParamsEncoded(c *gc.C) {
	catcher := api.UrlCatcher{}
	s.PatchValue(&api.WebsocketDial, catcher.RecordLocation)

	params := common.DebugLogParams{
		IncludeEntity: []string{"a", "b"},
		IncludeModule: []string{"c", "d"},
		IncludeLabel:  []string{"e", "f"},
		ExcludeEntity: []string{"g", "h"},
		ExcludeModule: []string{"i", "j"},
		ExcludeLabel:  []string{"k", "l"},
		Limit:         100,
		Backlog:       200,
		Level:         loggo.ERROR,
		Replay:        true,
		NoTail:        true,
		StartTime:     time.Date(2016, 11, 30, 11, 48, 0, 100, time.UTC),
	}

	urlValues := url.Values{
		"includeEntity": params.IncludeEntity,
		"includeModule": params.IncludeModule,
		"includeLabel":  params.IncludeLabel,
		"excludeEntity": params.ExcludeEntity,
		"excludeModule": params.ExcludeModule,
		"excludeLabel":  params.ExcludeLabel,
		"maxLines":      {"100"},
		"backlog":       {"200"},
		"level":         {"ERROR"},
		"replay":        {"true"},
		"noTail":        {"true"},
		"startTime":     {"2016-11-30T11:48:00.0000001Z"},
	}

	client := apiclient.NewClient(s.APIState, jtesting.NoopLogger{})
	_, err := client.WatchDebugLog(params)
	c.Assert(err, jc.ErrorIsNil)

	connectURL, err := url.Parse(catcher.Location())
	c.Assert(err, jc.ErrorIsNil)

	values := connectURL.Query()
	c.Assert(values, jc.DeepEquals, urlValues)
}

func (s *apiclientSuite) TestWatchDebugLogConnected(c *gc.C) {
	cl := apiclient.NewClient(s.APIState, jtesting.NoopLogger{})
	// Use the no tail option so we don't try to start a tailing cursor
	// on the oplog when there is no oplog configured in mongo as the tests
	// don't set up mongo in replicaset mode.
	messages, err := cl.WatchDebugLog(common.DebugLogParams{NoTail: true})
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(messages, gc.NotNil)
}

func (s *apiclientSuite) TestConnectStreamAtUUIDPath(c *gc.C) {
	catcher := api.UrlCatcher{}
	s.PatchValue(&api.WebsocketDial, catcher.RecordLocation)
	model, err := s.State.Model()
	c.Assert(err, jc.ErrorIsNil)
	info := s.APIInfo(c)
	info.ModelTag = model.ModelTag()
	apistate, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	defer apistate.Close()
	_, err = apistate.ConnectStream("/path", nil)
	c.Assert(err, jc.ErrorIsNil)
	connectURL, err := url.Parse(catcher.Location())
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(connectURL.Path, gc.Matches, fmt.Sprintf("/model/%s/path", model.UUID()))
}

func (s *apiclientSuite) TestOpenUsesModelUUIDPaths(c *gc.C) {
	info := s.APIInfo(c)

	// Passing in the correct model UUID should work
	model, err := s.State.Model()
	c.Assert(err, jc.ErrorIsNil)
	info.ModelTag = model.ModelTag()
	apistate, err := api.Open(info, api.DialOpts{})
	c.Assert(err, jc.ErrorIsNil)
	apistate.Close()

	// Passing in an unknown model UUID should fail with a known error
	info.ModelTag = names.NewModelTag("1eaf1e55-70ad-face-b007-70ad57001999")
	apistate, err = api.Open(info, api.DialOpts{})
	c.Assert(errors.Cause(err), gc.DeepEquals, &rpc.RequestError{
		Message: `unknown model: "1eaf1e55-70ad-face-b007-70ad57001999"`,
		Code:    "model not found",
	})
	c.Check(err, jc.Satisfies, params.IsCodeModelNotFound)
	c.Assert(apistate, gc.IsNil)
}

type clientDNSNameSuite struct {
	jjtesting.JujuConnSuite
}

var _ = gc.Suite(&clientDNSNameSuite{})

func (s *clientDNSNameSuite) SetUpTest(c *gc.C) {
	// Start an API server with a (non-working) autocert hostname,
	// so we can check that the PublicDNSName in the result goes
	// all the way through the layers.
	if s.ControllerConfigAttrs == nil {
		s.ControllerConfigAttrs = make(map[string]interface{})
	}
	s.ControllerConfigAttrs[controller.AutocertDNSNameKey] = "somewhere.example.com"
	s.JujuConnSuite.SetUpTest(c)
}

func (s *clientDNSNameSuite) TestPublicDNSName(c *gc.C) {
	apiInfo := s.APIInfo(c)
	conn, err := api.Open(apiInfo, api.DialOpts{})
	c.Assert(err, gc.IsNil)
	c.Assert(conn.PublicDNSName(), gc.Equals, "somewhere.example.com")
}

type fakeClock struct {
	clock.Clock

	mu    sync.Mutex
	now   time.Time
	waits []time.Duration
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.now.IsZero() {
		f.now = time.Now()
	}
	return f.now
}

func (f *fakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.waits = append(f.waits, d)
	f.now = f.now.Add(d)
	return time.After(0)
}

func (f *fakeClock) NewTimer(d time.Duration) clock.Timer {
	panic("NewTimer called on fakeClock - perhaps because fakeClock can't be used with DialOpts.Timeout")
}

func newRPCConnection(errs ...error) *fakeRPCConnection {
	conn := new(fakeRPCConnection)
	conn.stub.SetErrors(errs...)
	return conn
}

type fakeRPCConnection struct {
	stub     testing.Stub
	response interface{}
}

func (f *fakeRPCConnection) Dead() <-chan struct{} {
	return nil
}

func (f *fakeRPCConnection) Close() error {
	return nil
}

func (f *fakeRPCConnection) Call(req rpc.Request, params, response interface{}) error {
	f.stub.AddCall(req.Type+"."+req.Action, req.Version, params)
	if f.response != nil {
		rv := reflect.ValueOf(response)
		target := reflect.Indirect(rv)
		target.Set(reflect.Indirect(reflect.ValueOf(f.response)))
	}
	return f.stub.NextErr()
}

type redirectAPI struct {
	redirected       bool
	modelUUID        string
	redirectToHosts  []string
	redirectToCACert string
}

func (r *redirectAPI) Admin(id string) (*redirectAPIAdmin, error) {
	return &redirectAPIAdmin{r}, nil
}

type redirectAPIAdmin struct {
	r *redirectAPI
}

func (a *redirectAPIAdmin) Login(req params.LoginRequest) (params.LoginResult, error) {
	if a.r.modelUUID != "beef1beef1-0000-0000-000011112222" {
		return params.LoginResult{}, errors.New("logged into unexpected model")
	}
	a.r.redirected = true
	return params.LoginResult{}, params.Error{
		Message: "redirect",
		Code:    params.CodeRedirect,
	}
}

func (a *redirectAPIAdmin) RedirectInfo() (params.RedirectInfoResult, error) {
	if !a.r.redirected {
		return params.RedirectInfoResult{}, errors.New("not redirected")
	}

	hps, err := network.ParseProviderHostPorts(a.r.redirectToHosts...)
	if err != nil {
		panic(err)
	}
	return params.RedirectInfoResult{
		Servers: [][]params.HostPort{params.FromProviderHostPorts(hps)},
		CACert:  a.r.redirectToCACert,
	}, nil
}

func assertConnAddrForModel(c *gc.C, location, addr, modelUUID string) {
	c.Assert(location, gc.Equals, "wss://"+addr+"/model/"+modelUUID+"/api")
}

func assertConnAddrForRoot(c *gc.C, location, addr string) {
	c.Assert(location, gc.Matches, "wss://"+addr+"/api")
}

type fakeConn struct {
	closed chan struct{}
}

func (c fakeConn) Receive(x interface{}) error {
	return errors.New("no data available from fake connection")
}

func (c fakeConn) Send(x interface{}) error {
	return errors.New("cannot write to fake connection")
}

func (c fakeConn) Close() error {
	if c.closed != nil {
		close(c.closed)
	}
	return nil
}

// seqResolver returns an implementation of
// IPAddrResolver that maps the given addresses
// to sequential IP addresses 0.1.1.1, 0.2.2.2, etc.
func seqResolver(addrs ...string) api.IPAddrResolver {
	r := make(apitesting.IPAddrResolverMap)
	for i, addr := range addrs {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			panic(err)
		}
		r[host] = []string{fmt.Sprintf("0.%[1]d.%[1]d.%[1]d", i+1)}
	}
	return r
}

type dnsCacheMap map[string][]string

func (m dnsCacheMap) Lookup(host string) []string {
	return m[host]
}

func (m dnsCacheMap) Add(host string, ips []string) {
	m[host] = append([]string{}, ips...)
}

type loginTimeoutAPI struct {
	unblock chan chan struct{}
}

func (r *loginTimeoutAPI) Admin(id string) (*loginTimeoutAPIAdmin, error) {
	return &loginTimeoutAPIAdmin{r}, nil
}

type loginTimeoutAPIAdmin struct {
	r *loginTimeoutAPI
}

func (a *loginTimeoutAPIAdmin) Login(req params.LoginRequest) (params.LoginResult, error) {
	var unblocked chan struct{}
	select {
	case ch, ok := <-a.r.unblock:
		if !ok {
			return params.LoginResult{}, errors.New("abort")
		}
		unblocked = ch
	case <-time.After(jtesting.LongWait):
		return params.LoginResult{}, errors.New("timed out waiting to be unblocked")
	}
	select {
	case <-unblocked:
	case <-time.After(jtesting.LongWait):
		return params.LoginResult{}, errors.New("timed out sending on unblocked channel")
	}
	return params.LoginResult{}, errors.Errorf("login failed")
}
