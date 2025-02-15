// Copyright (C) The Arvados Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"git.arvados.org/arvados.git/sdk/go/arvados"
	"git.arvados.org/arvados.git/sdk/go/arvadosclient"
	"git.arvados.org/arvados.git/sdk/go/arvadostest"
	"git.arvados.org/arvados.git/sdk/go/ctxlog"
	"git.arvados.org/arvados.git/sdk/go/dispatch"
	"github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

// Gocheck boilerplate
func Test(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&TestSuite{})
var _ = Suite(&MockArvadosServerSuite{})

type TestSuite struct{}
type MockArvadosServerSuite struct{}

var initialArgs []string

func (s *TestSuite) SetUpSuite(c *C) {
	initialArgs = os.Args
	runningCmds = make(map[string]*exec.Cmd)
}

func (s *TestSuite) SetUpTest(c *C) {
	arvadostest.ResetDB(c)
	arvadostest.ResetEnv()
	args := []string{"crunch-dispatch-local"}
	os.Args = args
}

func (s *TestSuite) TearDownTest(c *C) {
	arvadostest.ResetEnv()
	arvadostest.ResetDB(c)
	os.Args = initialArgs
}

func (s *MockArvadosServerSuite) TearDownTest(c *C) {
	arvadostest.ResetEnv()
}

func (s *TestSuite) TestIntegration(c *C) {
	arv, err := arvadosclient.MakeArvadosClient()
	c.Assert(err, IsNil)

	echo := "echo"
	crunchRunCommand = echo

	ctx, cancel := context.WithCancel(ctxlog.Context(context.Background(), ctxlog.TestLogger(c)))
	dispatcher := dispatch.Dispatcher{
		Arv:        arv,
		PollPeriod: time.Second,
	}

	startCmd := func(container arvados.Container, cmd *exec.Cmd) error {
		dispatcher.UpdateState(container.UUID, "Running")
		dispatcher.UpdateState(container.UUID, "Complete")
		return cmd.Start()
	}

	cl := arvados.Cluster{Containers: arvados.ContainersConfig{RuntimeEngine: "docker"}}

	dispatcher.RunContainer = func(d *dispatch.Dispatcher, c arvados.Container, s <-chan arvados.Container) error {
		defer cancel()
		lr := LocalRun{startCmd, make(chan ResourceRequest), make(chan ResourceAlloc), ctx, &cl}
		go lr.throttle(logrus.StandardLogger())
		return lr.run(d, c, s)
	}

	err = dispatcher.Run(ctx)
	c.Assert(err, Equals, context.Canceled)

	// Wait for all running crunch jobs to complete / terminate
	waitGroup.Wait()

	// There should be no queued containers now
	params := arvadosclient.Dict{
		"filters": [][]string{{"state", "=", "Queued"}},
	}
	var containers arvados.ContainerList
	err = arv.List("containers", params, &containers)
	c.Check(err, IsNil)
	c.Assert(len(containers.Items), Equals, 0)

	// Previously "Queued" container should now be in "Complete" state
	var container arvados.Container
	err = arv.Get("containers", "zzzzz-dz642-queuedcontainer", nil, &container)
	c.Check(err, IsNil)
	c.Check(string(container.State), Equals, "Complete")
}

func (s *MockArvadosServerSuite) Test_APIErrorGettingContainers(c *C) {
	apiStubResponses := make(map[string]arvadostest.StubResponse)
	apiStubResponses["/arvados/v1/containers"] = arvadostest.StubResponse{500, string(`{}`)}

	testWithServerStub(c, apiStubResponses, "echo", "error getting count of containers")
}

func (s *MockArvadosServerSuite) Test_APIErrorUpdatingContainerState(c *C) {
	apiStubResponses := make(map[string]arvadostest.StubResponse)
	apiStubResponses["/arvados/v1/containers"] =
		arvadostest.StubResponse{200, string(`{"items_available":1, "items":[{"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx1","State":"Queued","Priority":1}]}`)}
	apiStubResponses["/arvados/v1/containers/zzzzz-dz642-xxxxxxxxxxxxxx1"] =
		arvadostest.StubResponse{500, string(`{}`)}

	testWithServerStub(c, apiStubResponses, "echo", "error locking container zzzzz-dz642-xxxxxxxxxxxxxx1")
}

func (s *MockArvadosServerSuite) Test_ContainerStillInRunningAfterRun(c *C) {
	apiStubResponses := make(map[string]arvadostest.StubResponse)
	apiStubResponses["/arvados/v1/containers"] =
		arvadostest.StubResponse{200, string(`{"items_available":1, "items":[{
"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx2",
"state":"Queued",
"priority":1,
"runtime_constraints": {
  "vcpus": 1,
  "ram": 1000000
}}]}`)}
	apiStubResponses["/arvados/v1/containers/zzzzz-dz642-xxxxxxxxxxxxxx2/lock"] =
		arvadostest.StubResponse{200, string(`{
"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx2",
"state":"Locked",
"priority":1,
"locked_by_uuid": "zzzzz-gj3su-000000000000000",
"runtime_constraints": {
  "vcpus": 1,
  "ram": 1000000
}}`)}
	apiStubResponses["/arvados/v1/containers/zzzzz-dz642-xxxxxxxxxxxxxx2"] =
		arvadostest.StubResponse{200, string(`{
"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx2",
"state":"Running",
"priority":1,
"locked_by_uuid": "zzzzz-gj3su-000000000000000",
"runtime_constraints": {
  "vcpus": 1,
  "ram": 1000000
}}`)}

	testWithServerStub(c, apiStubResponses, "echo",
		`after \\"echo\\" process termination, container state for zzzzz-dz642-xxxxxxxxxxxxxx2 is \\"Running\\"; updating it to \\"Cancelled\\"`)
}

func (s *MockArvadosServerSuite) Test_ErrorRunningContainer(c *C) {
	apiStubResponses := make(map[string]arvadostest.StubResponse)
	apiStubResponses["/arvados/v1/containers"] =
		arvadostest.StubResponse{200, string(`{"items_available":1, "items":[{
"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx3",
"state":"Queued",
"priority":1,
"runtime_constraints": {
  "vcpus": 1,
  "ram": 1000000
}}]}`)}

	apiStubResponses["/arvados/v1/containers/zzzzz-dz642-xxxxxxxxxxxxxx3/lock"] =
		arvadostest.StubResponse{200, string(`{
"uuid":"zzzzz-dz642-xxxxxxxxxxxxxx3",
"state":"Locked",
"priority":1,
"runtime_constraints": {
  "vcpus": 1,
  "ram": 1000000
}
}`)}

	testWithServerStub(c, apiStubResponses, "nosuchcommand", `error starting \\"nosuchcommand\\" for zzzzz-dz642-xxxxxxxxxxxxxx3`)
}

func testWithServerStub(c *C, apiStubResponses map[string]arvadostest.StubResponse, crunchCmd string, expected string) {
	apiStubResponses["/arvados/v1/api_client_authorizations/current"] =
		arvadostest.StubResponse{200, string(`{"uuid": "zzzzz-gj3su-000000000000000", "api_token": "xyz"}`)}

	apiStub := arvadostest.ServerStub{apiStubResponses}

	api := httptest.NewServer(&apiStub)
	defer api.Close()

	arv := &arvadosclient.ArvadosClient{
		Scheme:    "http",
		ApiServer: api.URL[7:],
		ApiToken:  "abc123",
		Client:    &http.Client{Transport: &http.Transport{}},
		Retries:   0,
	}

	buf := bytes.NewBuffer(nil)
	logger := ctxlog.TestLogger(c)
	logger.SetOutput(io.MultiWriter(buf, logger.Out))

	crunchRunCommand = crunchCmd

	ctx, cancel := context.WithCancel(ctxlog.Context(context.Background(), logger))
	defer cancel()
	dispatcher := dispatch.Dispatcher{
		Logger:     logger,
		Arv:        arv,
		PollPeriod: time.Second,
	}

	startCmd := func(container arvados.Container, cmd *exec.Cmd) error {
		dispatcher.UpdateState(container.UUID, "Running")
		dispatcher.UpdateState(container.UUID, "Complete")
		return cmd.Start()
	}

	cl := arvados.Cluster{Containers: arvados.ContainersConfig{RuntimeEngine: "docker"}}
	runningCmds = make(map[string]*exec.Cmd)

	dispatcher.RunContainer = func(d *dispatch.Dispatcher, c arvados.Container, s <-chan arvados.Container) error {
		defer cancel()
		lr := LocalRun{startCmd, make(chan ResourceRequest), make(chan ResourceAlloc), ctx, &cl}
		go lr.throttle(logrus.StandardLogger())
		return lr.run(d, c, s)
	}

	re := regexp.MustCompile(`(?ms).*` + expected + `.*`)
	go func() {
		for i := 0; i < 80 && !re.MatchString(buf.String()); i++ {
			time.Sleep(100 * time.Millisecond)
		}
		cancel()
	}()

	err := dispatcher.Run(ctx)
	c.Assert(err, Equals, context.Canceled)
	c.Check(buf.String(), Matches, `(?ms).*`+expected+`.*`)

	c.Logf("test finished, waiting for running crunch jobs to complete / terminate")
	waitGroup.Wait()
}
