// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package localdb

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.arvados.org/arvados.git/lib/controller/router"
	"git.arvados.org/arvados.git/lib/controller/rpc"
	"git.arvados.org/arvados.git/lib/crunchrun"
	"git.arvados.org/arvados.git/lib/ctrlctx"
	"git.arvados.org/arvados.git/sdk/go/arvados"
	"git.arvados.org/arvados.git/sdk/go/arvadosclient"
	"git.arvados.org/arvados.git/sdk/go/arvadostest"
	"git.arvados.org/arvados.git/sdk/go/auth"
	"git.arvados.org/arvados.git/sdk/go/ctxlog"
	"git.arvados.org/arvados.git/sdk/go/httpserver"
	"git.arvados.org/arvados.git/sdk/go/keepclient"
	"golang.org/x/crypto/ssh"
	check "gopkg.in/check.v1"
)

var _ = check.Suite(&ContainerGatewaySuite{})

type ContainerGatewaySuite struct {
	localdbSuite
	containerServices []*httpserver.Server
	reqCreateOptions  arvados.CreateOptions
	reqUUID           string
	ctrUUID           string
	srv               *httptest.Server
	gw                *crunchrun.Gateway
}

func (s *ContainerGatewaySuite) SetUpSuite(c *check.C) {
	s.localdbSuite.SetUpSuite(c)

	// Set up 10 http servers to play the role of services running
	// inside a container. (crunchrun.GatewayTargetStub will allow
	// our crunchrun.Gateway to connect to them directly on
	// localhost, rather than actually running them inside a
	// container.)
	for i := 0; i < 10; i++ {
		srv := &httpserver.Server{
			Addr: ":0",
			Server: http.Server{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					body := fmt.Sprintf("handled %s %s with Host %s", r.Method, r.URL.String(), r.Host)
					c.Logf("%s", body)
					w.Write([]byte(body))
				}),
			},
		}
		srv.Start()
		s.containerServices = append(s.containerServices, srv)
	}

	// s.containerServices[0] will be unlisted
	// s.containerServices[1] will be listed with access=public
	// s.containerServices[2,...] will be listed with access=private
	publishedPorts := make(map[string]arvados.RequestPublishedPort)
	for i, srv := range s.containerServices {
		access := arvados.PublishedPortAccessPrivate
		_, port, _ := net.SplitHostPort(srv.Addr)
		if i == 1 {
			access = arvados.PublishedPortAccessPublic
		}
		if i > 0 {
			publishedPorts[port] = arvados.RequestPublishedPort{
				Access: access,
				Label:  "port " + port,
			}
		}
	}

	s.reqCreateOptions = arvados.CreateOptions{
		Attrs: map[string]interface{}{
			"command":             []string{"echo", time.Now().Format(time.RFC3339Nano)},
			"container_count_max": 1,
			"container_image":     "arvados/apitestfixture:latest",
			"cwd":                 "/tmp",
			"environment":         map[string]string{},
			"output_path":         "/out",
			"priority":            1,
			"state":               arvados.ContainerRequestStateCommitted,
			"mounts": map[string]interface{}{
				"/out": map[string]interface{}{
					"kind":     "tmp",
					"capacity": 1000000,
				},
			},
			"runtime_constraints": map[string]interface{}{
				"vcpus": 1,
				"ram":   2,
			},
			"published_ports": publishedPorts}}
}

func (s *ContainerGatewaySuite) TearDownSuite(c *check.C) {
	for _, srv := range s.containerServices {
		go srv.Close()
	}
	s.containerServices = nil
	s.localdbSuite.TearDownSuite(c)
}

func (s *ContainerGatewaySuite) SetUpTest(c *check.C) {
	s.localdbSuite.SetUpTest(c)

	cr, err := s.localdb.ContainerRequestCreate(s.userctx, s.reqCreateOptions)
	c.Assert(err, check.IsNil)
	s.reqUUID = cr.UUID
	s.ctrUUID = cr.ContainerUUID

	h := hmac.New(sha256.New, []byte(s.cluster.SystemRootToken))
	fmt.Fprint(h, s.ctrUUID)
	authKey := fmt.Sprintf("%x", h.Sum(nil))

	rtr := router.New(s.localdb, router.Config{
		ContainerWebServicesURL: arvados.URL{Host: "*.containers.example.com"},
	})
	s.srv = httptest.NewUnstartedServer(httpserver.AddRequestIDs(httpserver.LogRequests(rtr)))
	s.srv.StartTLS()
	// the test setup doesn't use lib/service so
	// service.URLFromContext() returns nothing -- instead, this
	// is how we advertise our internal URL and enable
	// proxy-to-other-controller mode,
	forceInternalURLForTest = &arvados.URL{Scheme: "https", Host: s.srv.Listener.Addr().String()}
	s.cluster.Services.Controller.InternalURLs[*forceInternalURLForTest] = arvados.ServiceInstance{}
	ac := &arvados.Client{
		APIHost:   s.srv.Listener.Addr().String(),
		AuthToken: arvadostest.SystemRootToken,
		Insecure:  true,
	}
	s.gw = &crunchrun.Gateway{
		ContainerUUID: s.ctrUUID,
		AuthSecret:    authKey,
		Address:       "localhost:0",
		Log:           ctxlog.TestLogger(c),
		Target:        crunchrun.GatewayTargetStub{},
		ArvadosClient: ac,
	}
	c.Assert(s.gw.Start(), check.IsNil)

	rootctx := ctrlctx.NewWithToken(s.ctx, s.cluster, s.cluster.SystemRootToken)
	_, err = s.localdb.ContainerUpdate(rootctx, arvados.UpdateOptions{
		UUID: s.ctrUUID,
		Attrs: map[string]interface{}{
			"state": arvados.ContainerStateLocked}})
	c.Assert(err, check.IsNil)
	_, err = s.localdb.ContainerUpdate(rootctx, arvados.UpdateOptions{
		UUID: s.ctrUUID,
		Attrs: map[string]interface{}{
			"state":           arvados.ContainerStateRunning,
			"gateway_address": s.gw.Address}})
	c.Assert(err, check.IsNil)

	s.cluster.Containers.ShellAccess.Admin = true
	s.cluster.Containers.ShellAccess.User = true
	_, err = s.db.Exec(`update containers set interactive_session_started=$1 where uuid=$2`, false, s.ctrUUID)
	c.Check(err, check.IsNil)
}

func (s *ContainerGatewaySuite) TearDownTest(c *check.C) {
	forceProxyForTest = false
	if s.reqUUID != "" {
		_, err := s.localdb.ContainerRequestDelete(s.userctx, arvados.DeleteOptions{UUID: s.reqUUID})
		c.Check(err, check.IsNil)
	}
	if s.srv != nil {
		s.srv.Close()
		s.srv = nil
	}
	s.localdbSuite.TearDownTest(c)
}

func (s *ContainerGatewaySuite) TestConfig(c *check.C) {
	for _, trial := range []struct {
		configAdmin bool
		configUser  bool
		sendToken   string
		errorCode   int
	}{
		{true, true, arvadostest.ActiveTokenV2, 0},
		{true, false, arvadostest.ActiveTokenV2, 503},
		{false, true, arvadostest.ActiveTokenV2, 0},
		{false, false, arvadostest.ActiveTokenV2, 503},
		{true, true, arvadostest.AdminToken, 0},
		{true, false, arvadostest.AdminToken, 0},
		{false, true, arvadostest.AdminToken, 403},
		{false, false, arvadostest.AdminToken, 503},
	} {
		c.Logf("trial %#v", trial)
		s.cluster.Containers.ShellAccess.Admin = trial.configAdmin
		s.cluster.Containers.ShellAccess.User = trial.configUser
		ctx := ctrlctx.NewWithToken(s.ctx, s.cluster, trial.sendToken)
		sshconn, err := s.localdb.ContainerSSH(ctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
		if trial.errorCode == 0 {
			if !c.Check(err, check.IsNil) {
				continue
			}
			if !c.Check(sshconn.Conn, check.NotNil) {
				continue
			}
			sshconn.Conn.Close()
		} else {
			c.Check(err, check.NotNil)
			err, ok := err.(interface{ HTTPStatus() int })
			if c.Check(ok, check.Equals, true) {
				c.Check(err.HTTPStatus(), check.Equals, trial.errorCode)
			}
		}
	}
}

func (s *ContainerGatewaySuite) TestDirectTCP(c *check.C) {
	// Set up servers on a few TCP ports
	var addrs []string
	for i := 0; i < 3; i++ {
		ln, err := net.Listen("tcp", ":0")
		c.Assert(err, check.IsNil)
		defer ln.Close()
		addrs = append(addrs, ln.Addr().String())
		go func() {
			for {
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				var gotAddr string
				fmt.Fscanf(conn, "%s\n", &gotAddr)
				c.Logf("stub server listening at %s received string %q from remote %s", ln.Addr().String(), gotAddr, conn.RemoteAddr())
				if gotAddr == ln.Addr().String() {
					fmt.Fprintf(conn, "%s\n", ln.Addr().String())
				}
				conn.Close()
			}
		}()
	}

	c.Logf("connecting to %s", s.gw.Address)
	sshconn, err := s.localdb.ContainerSSH(s.userctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
	c.Assert(err, check.IsNil)
	c.Assert(sshconn.Conn, check.NotNil)
	defer sshconn.Conn.Close()
	conn, chans, reqs, err := ssh.NewClientConn(sshconn.Conn, "zzzz-dz642-abcdeabcdeabcde", &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil },
	})
	c.Assert(err, check.IsNil)
	client := ssh.NewClient(conn, chans, reqs)
	for _, expectAddr := range addrs {
		_, port, err := net.SplitHostPort(expectAddr)
		c.Assert(err, check.IsNil)

		c.Logf("trying foo:%s", port)
		{
			conn, err := client.Dial("tcp", "foo:"+port)
			c.Assert(err, check.IsNil)
			conn.SetDeadline(time.Now().Add(time.Second))
			buf, err := ioutil.ReadAll(conn)
			c.Check(err, check.IsNil)
			c.Check(string(buf), check.Equals, "")
		}

		c.Logf("trying localhost:%s", port)
		{
			conn, err := client.Dial("tcp", "localhost:"+port)
			c.Assert(err, check.IsNil)
			conn.SetDeadline(time.Now().Add(time.Second))
			conn.Write([]byte(expectAddr + "\n"))
			var gotAddr string
			fmt.Fscanf(conn, "%s\n", &gotAddr)
			c.Check(gotAddr, check.Equals, expectAddr)
		}
	}
}

// Connect to crunch-run container gateway directly, using container
// UUID.
func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Direct(c *check.C) {
	s.testContainerHTTPProxy(c, s.ctrUUID)
}

// Connect to crunch-run container gateway directly, using container
// request UUID.
func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Direct_ContainerRequestUUID(c *check.C) {
	s.testContainerHTTPProxy(c, s.reqUUID)
}

// Connect through a tunnel terminated at this controller process.
func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Tunnel(c *check.C) {
	s.gw = s.setupGatewayWithTunnel(c)
	s.testContainerHTTPProxy(c, s.ctrUUID)
}

// Connect through a tunnel terminated at a different controller
// process.
func (s *ContainerGatewaySuite) TestContainerHTTPProxy_ProxyTunnel(c *check.C) {
	forceProxyForTest = true
	s.gw = s.setupGatewayWithTunnel(c)
	s.testContainerHTTPProxy(c, s.ctrUUID)
}

func (s *ContainerGatewaySuite) testContainerHTTPProxy(c *check.C, targetUUID string) {
	testMethods := []string{"GET", "POST", "PATCH", "OPTIONS", "DELETE"}

	var wg sync.WaitGroup
	for idx, srv := range s.containerServices {
		idx, srv := idx, srv
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Logf("sending request to %s via %s", srv.Addr, s.gw.Address)
			method := testMethods[idx%len(testMethods)]
			_, port, err := net.SplitHostPort(srv.Addr)
			c.Assert(err, check.IsNil)
			vhost := targetUUID + "-" + port + ".containers.example.com"
			req, err := http.NewRequest(method, "https://"+vhost+"/via-"+s.gw.Address, nil)
			c.Assert(err, check.IsNil)
			// Token is already passed to
			// ContainerHTTPProxy() call in s.userctx, but
			// we also need to add an auth cookie to the
			// http request: if the request gets passed
			// through http (see forceProxyForTest), the
			// target router will start with a fresh
			// context and load tokens from the request.
			req.AddCookie(&http.Cookie{
				Name:  "arvados_api_token",
				Value: auth.EncodeTokenCookie([]byte(arvadostest.ActiveTokenV2)),
			})
			handler, err := s.localdb.ContainerHTTPProxy(s.userctx, arvados.ContainerHTTPProxyOptions{
				Target:  fmt.Sprintf("%s-%s", targetUUID, port),
				Request: req,
			})
			c.Assert(err, check.IsNil)
			rw := httptest.NewRecorder()
			handler.ServeHTTP(rw, req)
			resp := rw.Result()
			c.Check(resp.StatusCode, check.Equals, http.StatusOK)
			body, err := io.ReadAll(resp.Body)
			c.Assert(err, check.IsNil)
			c.Check(string(body), check.Matches, `handled `+method+` /via-.* with Host \Q`+vhost+`\E`)
		}()
	}
	wg.Wait()
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_NoToken_Unlisted(c *check.C) {
	s.testContainerHTTPProxyError(c, 0, "", http.StatusUnauthorized)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_NoToken_Private(c *check.C) {
	s.testContainerHTTPProxyError(c, 2, "", http.StatusUnauthorized)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_InvalidToken(c *check.C) {
	s.testContainerHTTPProxyError(c, 0, arvadostest.ActiveTokenV2+"bogus", http.StatusUnauthorized)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_AnonymousToken_Unlisted(c *check.C) {
	s.testContainerHTTPProxyError(c, 0, arvadostest.AnonymousToken, http.StatusNotFound)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_AnonymousToken_Private(c *check.C) {
	s.testContainerHTTPProxyError(c, 2, arvadostest.AnonymousToken, http.StatusNotFound)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_CRsDifferentUsers(c *check.C) {
	rootctx := ctrlctx.NewWithToken(s.ctx, s.cluster, s.cluster.SystemRootToken)
	cr, err := s.localdb.ContainerRequestCreate(rootctx, s.reqCreateOptions)
	defer s.localdb.ContainerRequestDelete(rootctx, arvados.DeleteOptions{UUID: cr.UUID})
	c.Assert(err, check.IsNil)
	c.Assert(cr.ContainerUUID, check.Equals, s.ctrUUID)
	s.testContainerHTTPProxyError(c, 0, arvadostest.ActiveTokenV2, http.StatusForbidden)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxyError_ContainerNotReadable(c *check.C) {
	s.testContainerHTTPProxyError(c, 0, arvadostest.SpectatorToken, http.StatusNotFound)
}

func (s *ContainerGatewaySuite) testContainerHTTPProxyError(c *check.C, svcIdx int, token string, expectCode int) {
	_, svcPort, err := net.SplitHostPort(s.containerServices[svcIdx].Addr)
	c.Assert(err, check.IsNil)
	ctx := ctrlctx.NewWithToken(s.ctx, s.cluster, token)
	vhost := s.ctrUUID + "-" + svcPort + ".containers.example.com"
	req, err := http.NewRequest("GET", "https://"+vhost+"/via-"+s.gw.Address, nil)
	c.Assert(err, check.IsNil)
	_, err = s.localdb.ContainerHTTPProxy(ctx, arvados.ContainerHTTPProxyOptions{
		Target:  fmt.Sprintf("%s-%s", s.ctrUUID, svcPort),
		Request: req,
	})
	c.Check(err, check.NotNil)
	var se httpserver.HTTPStatusError
	c.Assert(errors.As(err, &se), check.Equals, true)
	c.Check(se.HTTPStatus(), check.Equals, expectCode)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_CookieAuth(c *check.C) {
	s.testContainerHTTPProxyUsingCurl(c, 0, arvadostest.ActiveTokenV2, "GET", "/foobar")
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_CookieAuth_POST(c *check.C) {
	s.testContainerHTTPProxyUsingCurl(c, 0, arvadostest.ActiveTokenV2, "POST", "/foobar")
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_QueryAuth(c *check.C) {
	s.testContainerHTTPProxyUsingCurl(c, 0, "", "GET", "/foobar?arvados_api_token="+arvadostest.ActiveTokenV2)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_QueryAuth_Tunnel(c *check.C) {
	s.gw = s.setupGatewayWithTunnel(c)
	s.testContainerHTTPProxyUsingCurl(c, 0, "", "GET", "/foobar?arvados_api_token="+arvadostest.ActiveTokenV2)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_QueryAuth_ProxyTunnel(c *check.C) {
	forceProxyForTest = true
	s.gw = s.setupGatewayWithTunnel(c)
	s.testContainerHTTPProxyUsingCurl(c, 0, "", "GET", "/foobar?arvados_api_token="+arvadostest.ActiveTokenV2)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_Anonymous(c *check.C) {
	s.testContainerHTTPProxyUsingCurl(c, 1, "", "GET", "/foobar")
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_Anonymous_OPTIONS(c *check.C) {
	s.testContainerHTTPProxyUsingCurl(c, 1, "", "OPTIONS", "/foobar")
}

// Check other query parameters are preserved in the
// redirect-with-cookie.
//
// Note the original request has "?baz&baz&..." and this changes to
// "?baz=&baz=&..." in the redirect location.  We trust the target
// service won't be sensitive to this difference.
func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_QueryAuth_PreserveQuery(c *check.C) {
	body := s.testContainerHTTPProxyUsingCurl(c, 0, "", "GET", "/foobar?baz&baz&arvados_api_token="+arvadostest.ActiveTokenV2+"&waz=quux")
	c.Check(body, check.Matches, `handled GET /foobar\?baz=&baz=&waz=quux with Host `+s.ctrUUID+`.*`)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_Curl_Patch(c *check.C) {
	body := s.testContainerHTTPProxyUsingCurl(c, 0, arvadostest.ActiveTokenV2, "PATCH", "/foobar")
	c.Check(body, check.Matches, `handled PATCH /foobar with Host `+s.ctrUUID+`.*`)
}

func (s *ContainerGatewaySuite) testContainerHTTPProxyUsingCurl(c *check.C, svcIdx int, cookietoken, method, path string) string {
	_, svcPort, err := net.SplitHostPort(s.containerServices[svcIdx].Addr)
	c.Assert(err, check.IsNil)

	vhost, err := url.Parse(s.srv.URL)
	c.Assert(err, check.IsNil)
	controllerHost := vhost.Host
	vhost.Host = s.ctrUUID + "-" + svcPort + ".containers.example.com"
	target, err := vhost.Parse(path)
	c.Assert(err, check.IsNil)

	tempdir, err := ioutil.TempDir("", "localdb-test-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempdir)

	cmd := exec.Command("curl")
	if cookietoken != "" {
		cmd.Args = append(cmd.Args, "--cookie", "arvados_api_token="+string(auth.EncodeTokenCookie([]byte(cookietoken))))
	} else {
		cmd.Args = append(cmd.Args, "--cookie-jar", filepath.Join(tempdir, "cookie.jar"))
	}
	if method != "GET" {
		cmd.Args = append(cmd.Args, "--request", method)
	}
	cmd.Args = append(cmd.Args, "--silent", "--insecure", "--location", "--connect-to", vhost.Hostname()+":443:"+controllerHost, target.String())
	cmd.Dir = tempdir
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, check.Equals, nil)
	cmd.Stderr = cmd.Stdout
	c.Logf("cmd: %v", cmd.Args)
	go cmd.Start()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, stdout)
	c.Check(err, check.Equals, nil)
	err = cmd.Wait()
	c.Check(err, check.Equals, nil)
	c.Check(buf.String(), check.Matches, `handled `+method+` /.*`)
	return buf.String()
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_PublishedPortByName_ProxyTunnel(c *check.C) {
	forceProxyForTest = true
	s.gw = s.setupGatewayWithTunnel(c)
	s.testContainerHTTPProxy_PublishedPortByName(c)
}

func (s *ContainerGatewaySuite) TestContainerHTTPProxy_PublishedPortByName(c *check.C) {
	s.testContainerHTTPProxy_PublishedPortByName(c)
}

func (s *ContainerGatewaySuite) testContainerHTTPProxy_PublishedPortByName(c *check.C) {
	srv := s.containerServices[1]
	_, port, _ := net.SplitHostPort(srv.Addr)
	portnum, err := strconv.Atoi(port)
	c.Assert(err, check.IsNil)
	namelink, err := s.localdb.LinkCreate(s.userctx, arvados.CreateOptions{
		Attrs: map[string]interface{}{
			"link_class": "published_port",
			"name":       "warthogfacedbuffoon",
			"head_uuid":  s.reqUUID,
			"properties": map[string]interface{}{
				"port": portnum}}})
	c.Assert(err, check.IsNil)
	defer s.localdb.LinkDelete(s.userctx, arvados.DeleteOptions{UUID: namelink.UUID})

	vhost := namelink.Name + ".containers.example.com"
	req, err := http.NewRequest("METHOD", "https://"+vhost+"/path", nil)
	c.Assert(err, check.IsNil)
	// Token is already passed to ContainerHTTPProxy() call in
	// s.userctx, but we also need to add an auth cookie to the
	// http request: if the request gets passed through http (see
	// forceProxyForTest), the target router will start with a
	// fresh context and load tokens from the request.
	req.AddCookie(&http.Cookie{
		Name:  "arvados_api_token",
		Value: auth.EncodeTokenCookie([]byte(arvadostest.ActiveTokenV2)),
	})
	handler, err := s.localdb.ContainerHTTPProxy(s.userctx, arvados.ContainerHTTPProxyOptions{
		Target:  namelink.Name,
		Request: req,
	})
	c.Assert(err, check.IsNil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)
	resp := rw.Result()
	c.Check(resp.StatusCode, check.Equals, http.StatusOK)
	body, err := io.ReadAll(resp.Body)
	c.Assert(err, check.IsNil)
	c.Check(string(body), check.Matches, `handled METHOD /path with Host \Q`+vhost+`\E`)
}

func (s *ContainerGatewaySuite) setupLogCollection(c *check.C) {
	files := map[string]string{
		"stderr.txt":   "hello world\n",
		"a/b/c/d.html": "<html></html>\n",
	}
	client := arvados.NewClientFromEnv()
	ac, err := arvadosclient.New(client)
	c.Assert(err, check.IsNil)
	kc, err := keepclient.MakeKeepClient(ac)
	c.Assert(err, check.IsNil)
	cfs, err := (&arvados.Collection{}).FileSystem(client, kc)
	c.Assert(err, check.IsNil)
	for name, content := range files {
		for i, ch := range name {
			if ch == '/' {
				err := cfs.Mkdir("/"+name[:i], 0777)
				c.Assert(err, check.IsNil)
			}
		}
		f, err := cfs.OpenFile("/"+name, os.O_CREATE|os.O_WRONLY, 0777)
		c.Assert(err, check.IsNil)
		f.Write([]byte(content))
		err = f.Close()
		c.Assert(err, check.IsNil)
	}
	cfs.Sync()
	s.gw.LogCollection = cfs
}

func (s *ContainerGatewaySuite) saveLogAndCloseGateway(c *check.C) {
	rootctx := ctrlctx.NewWithToken(s.ctx, s.cluster, s.cluster.SystemRootToken)
	txt, err := s.gw.LogCollection.MarshalManifest(".")
	c.Assert(err, check.IsNil)
	coll, err := s.localdb.CollectionCreate(rootctx, arvados.CreateOptions{
		Attrs: map[string]interface{}{
			"manifest_text": txt,
		}})
	c.Assert(err, check.IsNil)
	_, err = s.localdb.ContainerUpdate(rootctx, arvados.UpdateOptions{
		UUID: s.ctrUUID,
		Attrs: map[string]interface{}{
			"state":     arvados.ContainerStateComplete,
			"exit_code": 0,
			"log":       coll.PortableDataHash,
		}})
	c.Assert(err, check.IsNil)
	updatedReq, err := s.localdb.ContainerRequestGet(rootctx, arvados.GetOptions{UUID: s.reqUUID})
	c.Assert(err, check.IsNil)
	c.Logf("container request log UUID is %s", updatedReq.LogUUID)
	crLog, err := s.localdb.CollectionGet(rootctx, arvados.GetOptions{UUID: updatedReq.LogUUID, Select: []string{"manifest_text"}})
	c.Assert(err, check.IsNil)
	c.Logf("collection log manifest:\n%s", crLog.ManifestText)
	// Ensure localdb can't circumvent the keep-web proxy test by
	// getting content from the container gateway.
	s.gw.LogCollection = nil
}

func (s *ContainerGatewaySuite) TestContainerRequestLogViaTunnel(c *check.C) {
	forceProxyForTest = true
	s.gw = s.setupGatewayWithTunnel(c)
	s.setupLogCollection(c)

	for _, broken := range []bool{false, true} {
		c.Logf("broken=%v", broken)

		if broken {
			delete(s.cluster.Services.Controller.InternalURLs, *forceInternalURLForTest)
		}

		r, err := http.NewRequestWithContext(s.userctx, "GET", "https://controller.example/arvados/v1/container_requests/"+s.reqUUID+"/log/"+s.ctrUUID+"/stderr.txt", nil)
		c.Assert(err, check.IsNil)
		r.Header.Set("Authorization", "Bearer "+arvadostest.ActiveTokenV2)
		handler, err := s.localdb.ContainerRequestLog(s.userctx, arvados.ContainerLogOptions{
			UUID: s.reqUUID,
			WebDAVOptions: arvados.WebDAVOptions{
				Method: "GET",
				Header: r.Header,
				Path:   "/" + s.ctrUUID + "/stderr.txt",
			},
		})
		if broken {
			c.Check(err, check.ErrorMatches, `.*tunnel endpoint is invalid.*`)
			continue
		}
		c.Check(err, check.IsNil)
		c.Assert(handler, check.NotNil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		resp := rec.Result()
		c.Check(resp.StatusCode, check.Equals, http.StatusOK)
		buf, err := ioutil.ReadAll(resp.Body)
		c.Check(err, check.IsNil)
		c.Check(string(buf), check.Equals, "hello world\n")
	}
}

func (s *ContainerGatewaySuite) TestContainerRequestLogViaGateway(c *check.C) {
	s.setupLogCollection(c)
	s.testContainerRequestLog(c)
}

func (s *ContainerGatewaySuite) TestContainerRequestLogViaKeepWeb(c *check.C) {
	s.setupLogCollection(c)
	s.saveLogAndCloseGateway(c)
	s.testContainerRequestLog(c)
}

func (s *ContainerGatewaySuite) testContainerRequestLog(c *check.C) {
	for _, trial := range []struct {
		method          string
		path            string
		header          http.Header
		unauthenticated bool
		expectStatus    int
		expectBodyRe    string
		expectHeader    http.Header
	}{
		{
			method:       "GET",
			path:         s.ctrUUID + "/stderr.txt",
			expectStatus: http.StatusOK,
			expectBodyRe: "hello world\n",
			expectHeader: http.Header{
				"Content-Type": {"text/plain; charset=utf-8"},
			},
		},
		{
			method: "GET",
			path:   s.ctrUUID + "/stderr.txt",
			header: http.Header{
				"Range": {"bytes=-6"},
			},
			expectStatus: http.StatusPartialContent,
			expectBodyRe: "world\n",
			expectHeader: http.Header{
				"Content-Type":  {"text/plain; charset=utf-8"},
				"Content-Range": {"bytes 6-11/12"},
			},
		},
		{
			method:       "OPTIONS",
			path:         s.ctrUUID + "/stderr.txt",
			expectStatus: http.StatusOK,
			expectBodyRe: "",
			expectHeader: http.Header{
				"Dav":   {"1, 2"},
				"Allow": {"OPTIONS, LOCK, GET, HEAD, POST, DELETE, PROPPATCH, COPY, MOVE, UNLOCK, PROPFIND, PUT"},
			},
		},
		{
			method:          "OPTIONS",
			path:            s.ctrUUID + "/stderr.txt",
			unauthenticated: true,
			header: http.Header{
				"Access-Control-Request-Method": {"POST"},
			},
			expectStatus: http.StatusOK,
			expectBodyRe: "",
			expectHeader: http.Header{
				"Access-Control-Allow-Headers": {"Authorization, Content-Type, Range, Depth, Destination, If, Lock-Token, Overwrite, Timeout, Cache-Control"},
				"Access-Control-Allow-Methods": {"COPY, DELETE, GET, LOCK, MKCOL, MOVE, OPTIONS, POST, PROPFIND, PROPPATCH, PUT, RMCOL, UNLOCK"},
				"Access-Control-Allow-Origin":  {"*"},
				"Access-Control-Max-Age":       {"86400"},
			},
		},
		{
			method:       "PROPFIND",
			path:         s.ctrUUID + "/",
			expectStatus: http.StatusMultiStatus,
			expectBodyRe: `.*\Q<D:displayname>stderr.txt</D:displayname>\E.*>\n?`,
			expectHeader: http.Header{
				"Content-Type": {"text/xml; charset=utf-8"},
			},
		},
		{
			method:       "PROPFIND",
			path:         s.ctrUUID,
			expectStatus: http.StatusMultiStatus,
			expectBodyRe: `.*\Q<D:displayname>stderr.txt</D:displayname>\E.*>\n?`,
			expectHeader: http.Header{
				"Content-Type": {"text/xml; charset=utf-8"},
			},
		},
		{
			method:       "PROPFIND",
			path:         s.ctrUUID + "/a/b/c/",
			expectStatus: http.StatusMultiStatus,
			expectBodyRe: `.*\Q<D:displayname>d.html</D:displayname>\E.*>\n?`,
			expectHeader: http.Header{
				"Content-Type": {"text/xml; charset=utf-8"},
			},
		},
		{
			method:       "GET",
			path:         s.ctrUUID + "/a/b/c/d.html",
			expectStatus: http.StatusOK,
			expectBodyRe: "<html></html>\n",
			expectHeader: http.Header{
				"Content-Type": {"text/html; charset=utf-8"},
			},
		},
	} {
		c.Logf("trial %#v", trial)
		ctx := s.userctx
		if trial.unauthenticated {
			ctx = auth.NewContext(context.Background(), auth.CredentialsFromRequest(&http.Request{URL: &url.URL{}, Header: http.Header{}}))
		}
		r, err := http.NewRequestWithContext(ctx, trial.method, "https://controller.example/arvados/v1/container_requests/"+s.reqUUID+"/log/"+trial.path, nil)
		c.Assert(err, check.IsNil)
		for k := range trial.header {
			r.Header.Set(k, trial.header.Get(k))
		}
		handler, err := s.localdb.ContainerRequestLog(ctx, arvados.ContainerLogOptions{
			UUID: s.reqUUID,
			WebDAVOptions: arvados.WebDAVOptions{
				Method: trial.method,
				Header: r.Header,
				Path:   "/" + trial.path,
			},
		})
		c.Assert(err, check.IsNil)
		c.Assert(handler, check.NotNil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, r)
		resp := rec.Result()
		c.Check(resp.StatusCode, check.Equals, trial.expectStatus)
		for k := range trial.expectHeader {
			c.Check(resp.Header[k], check.DeepEquals, trial.expectHeader[k])
		}
		buf, err := ioutil.ReadAll(resp.Body)
		c.Check(err, check.IsNil)
		c.Check(string(buf), check.Matches, trial.expectBodyRe)
	}
}

func (s *ContainerGatewaySuite) TestContainerRequestLogViaCadaver(c *check.C) {
	s.setupLogCollection(c)

	out := s.runCadaver(c, arvadostest.ActiveToken, "/arvados/v1/container_requests/"+s.reqUUID+"/log/"+s.ctrUUID, "ls")
	c.Check(out, check.Matches, `(?ms).*stderr\.txt\s+12\s.*`)
	c.Check(out, check.Matches, `(?ms).*a\s+0\s.*`)

	out = s.runCadaver(c, arvadostest.ActiveTokenV2, "/arvados/v1/container_requests/"+s.reqUUID+"/log/"+s.ctrUUID, "get stderr.txt")
	c.Check(out, check.Matches, `(?ms).*Downloading .* to stderr\.txt: .* succeeded\..*`)

	s.saveLogAndCloseGateway(c)

	out = s.runCadaver(c, arvadostest.ActiveTokenV2, "/arvados/v1/container_requests/"+s.reqUUID+"/log/"+s.ctrUUID, "get stderr.txt")
	c.Check(out, check.Matches, `(?ms).*Downloading .* to stderr\.txt: .* succeeded\..*`)
}

func (s *ContainerGatewaySuite) runCadaver(c *check.C, password, path, stdin string) string {
	// Replace s.srv with an HTTP server, otherwise cadaver will
	// just fail on TLS cert verification.
	s.srv.Close()
	rtr := router.New(s.localdb, router.Config{})
	s.srv = httptest.NewUnstartedServer(httpserver.AddRequestIDs(httpserver.LogRequests(rtr)))
	s.srv.Start()

	tempdir, err := ioutil.TempDir("", "localdb-test-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempdir)

	cmd := exec.Command("cadaver", s.srv.URL+path)
	if password != "" {
		cmd.Env = append(os.Environ(), "HOME="+tempdir)
		f, err := os.OpenFile(filepath.Join(tempdir, ".netrc"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		c.Assert(err, check.IsNil)
		_, err = fmt.Fprintf(f, "default login none password %s\n", password)
		c.Assert(err, check.IsNil)
		c.Assert(f.Close(), check.IsNil)
	}
	cmd.Stdin = bytes.NewBufferString(stdin)
	cmd.Dir = tempdir
	stdout, err := cmd.StdoutPipe()
	c.Assert(err, check.Equals, nil)
	cmd.Stderr = cmd.Stdout
	c.Logf("cmd: %v", cmd.Args)
	go cmd.Start()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, stdout)
	c.Check(err, check.Equals, nil)
	err = cmd.Wait()
	c.Check(err, check.Equals, nil)
	return buf.String()
}

func (s *ContainerGatewaySuite) TestConnect(c *check.C) {
	c.Logf("connecting to %s", s.gw.Address)
	sshconn, err := s.localdb.ContainerSSH(s.userctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
	c.Assert(err, check.IsNil)
	c.Assert(sshconn.Conn, check.NotNil)
	defer sshconn.Conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Receive text banner
		buf := make([]byte, 12)
		_, err := io.ReadFull(sshconn.Conn, buf)
		c.Check(err, check.IsNil)
		c.Check(string(buf), check.Equals, "SSH-2.0-Go\r\n")

		// Send text banner
		_, err = sshconn.Conn.Write([]byte("SSH-2.0-Fake\r\n"))
		c.Check(err, check.IsNil)

		// Receive binary
		_, err = io.ReadFull(sshconn.Conn, buf[:4])
		c.Check(err, check.IsNil)

		// If we can get this far into an SSH handshake...
		c.Logf("was able to read %x -- success, tunnel is working", buf[:4])
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		c.Fail()
	}
	ctr, err := s.localdb.ContainerGet(s.userctx, arvados.GetOptions{UUID: s.ctrUUID})
	c.Check(err, check.IsNil)
	c.Check(ctr.InteractiveSessionStarted, check.Equals, true)
}

func (s *ContainerGatewaySuite) TestConnectFail_NoToken(c *check.C) {
	ctx := ctrlctx.NewWithToken(s.ctx, s.cluster, "")
	_, err := s.localdb.ContainerSSH(ctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
	c.Check(err, check.ErrorMatches, `.* 401 .*`)
}

func (s *ContainerGatewaySuite) TestConnectFail_AnonymousToken(c *check.C) {
	ctx := ctrlctx.NewWithToken(s.ctx, s.cluster, arvadostest.AnonymousToken)
	_, err := s.localdb.ContainerSSH(ctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
	c.Check(err, check.ErrorMatches, `.* 404 .*`)
}

func (s *ContainerGatewaySuite) TestCreateTunnel(c *check.C) {
	// no AuthSecret
	conn, err := s.localdb.ContainerGatewayTunnel(s.userctx, arvados.ContainerGatewayTunnelOptions{
		UUID: s.ctrUUID,
	})
	c.Check(err, check.ErrorMatches, `authentication error`)
	c.Check(conn.Conn, check.IsNil)

	// bogus AuthSecret
	conn, err = s.localdb.ContainerGatewayTunnel(s.userctx, arvados.ContainerGatewayTunnelOptions{
		UUID:       s.ctrUUID,
		AuthSecret: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	})
	c.Check(err, check.ErrorMatches, `authentication error`)
	c.Check(conn.Conn, check.IsNil)

	// good AuthSecret
	conn, err = s.localdb.ContainerGatewayTunnel(s.userctx, arvados.ContainerGatewayTunnelOptions{
		UUID:       s.ctrUUID,
		AuthSecret: s.gw.AuthSecret,
	})
	c.Check(err, check.IsNil)
	c.Check(conn.Conn, check.NotNil)
}

func (s *ContainerGatewaySuite) TestConnectThroughTunnelWithProxyOK(c *check.C) {
	forceProxyForTest = true
	s.testConnectThroughTunnel(c, "")
}

func (s *ContainerGatewaySuite) TestConnectThroughTunnelWithProxyError(c *check.C) {
	forceProxyForTest = true
	delete(s.cluster.Services.Controller.InternalURLs, *forceInternalURLForTest)
	s.testConnectThroughTunnel(c, `.*tunnel endpoint is invalid.*`)
}

func (s *ContainerGatewaySuite) TestConnectThroughTunnelNoProxyOK(c *check.C) {
	s.testConnectThroughTunnel(c, "")
}

func (s *ContainerGatewaySuite) setupGatewayWithTunnel(c *check.C) *crunchrun.Gateway {
	rootctx := ctrlctx.NewWithToken(s.ctx, s.cluster, s.cluster.SystemRootToken)
	// Until the tunnel starts up, set gateway_address to a value
	// that can't work. We want to ensure the only way we can
	// reach the gateway is through the tunnel.
	tungw := &crunchrun.Gateway{
		ContainerUUID: s.ctrUUID,
		AuthSecret:    s.gw.AuthSecret,
		Log:           ctxlog.TestLogger(c),
		Target:        crunchrun.GatewayTargetStub{},
		ArvadosClient: s.gw.ArvadosClient,
		UpdateTunnelURL: func(url string) {
			c.Logf("UpdateTunnelURL(%q)", url)
			gwaddr := "tunnel " + url
			s.localdb.ContainerUpdate(rootctx, arvados.UpdateOptions{
				UUID: s.ctrUUID,
				Attrs: map[string]interface{}{
					"gateway_address": gwaddr}})
		},
	}
	c.Assert(tungw.Start(), check.IsNil)

	// We didn't supply an external hostname in the Address field,
	// so Start() should assign a local address.
	host, _, err := net.SplitHostPort(tungw.Address)
	c.Assert(err, check.IsNil)
	c.Check(host, check.Equals, "127.0.0.1")

	_, err = s.localdb.ContainerUpdate(rootctx, arvados.UpdateOptions{
		UUID: s.ctrUUID,
		Attrs: map[string]interface{}{
			"state": arvados.ContainerStateRunning,
		}})
	c.Assert(err, check.IsNil)

	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second / 2) {
		ctr, err := s.localdb.ContainerGet(s.userctx, arvados.GetOptions{UUID: s.ctrUUID})
		c.Assert(err, check.IsNil)
		c.Check(ctr.InteractiveSessionStarted, check.Equals, false)
		c.Logf("ctr.GatewayAddress == %s", ctr.GatewayAddress)
		if strings.HasPrefix(ctr.GatewayAddress, "tunnel ") {
			break
		}
	}
	return tungw
}

func (s *ContainerGatewaySuite) testConnectThroughTunnel(c *check.C, expectErrorMatch string) {
	s.setupGatewayWithTunnel(c)
	c.Log("connecting to gateway through tunnel")
	arpc := rpc.NewConn("", &url.URL{Scheme: "https", Host: s.gw.ArvadosClient.APIHost}, true, rpc.PassthroughTokenProvider)
	sshconn, err := arpc.ContainerSSH(s.userctx, arvados.ContainerSSHOptions{UUID: s.ctrUUID})
	if expectErrorMatch != "" {
		c.Check(err, check.ErrorMatches, expectErrorMatch)
		return
	}
	c.Assert(err, check.IsNil)
	c.Assert(sshconn.Conn, check.NotNil)
	defer sshconn.Conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		// Receive text banner
		buf := make([]byte, 12)
		_, err := io.ReadFull(sshconn.Conn, buf)
		c.Check(err, check.IsNil)
		c.Check(string(buf), check.Equals, "SSH-2.0-Go\r\n")

		// Send text banner
		_, err = sshconn.Conn.Write([]byte("SSH-2.0-Fake\r\n"))
		c.Check(err, check.IsNil)

		// Receive binary
		_, err = io.ReadFull(sshconn.Conn, buf[:4])
		c.Check(err, check.IsNil)

		// If we can get this far into an SSH handshake...
		c.Logf("was able to read %x -- success, tunnel is working", buf[:4])
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		c.Fail()
	}
	ctr, err := s.localdb.ContainerGet(s.userctx, arvados.GetOptions{UUID: s.ctrUUID})
	c.Check(err, check.IsNil)
	c.Check(ctr.InteractiveSessionStarted, check.Equals, true)
}
