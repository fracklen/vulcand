package command

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/gorilla/mux"
	log "github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/gotools-log"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/vulcan/route/hostroute"
	. "github.com/mailgun/vulcand/Godeps/_workspace/src/gopkg.in/check.v1"
	"github.com/mailgun/vulcand/adapter"
	"github.com/mailgun/vulcand/api"
	. "github.com/mailgun/vulcand/backend"
	"github.com/mailgun/vulcand/backend/membackend"
	"github.com/mailgun/vulcand/configure"
	"github.com/mailgun/vulcand/plugin/registry"
)

func TestVulcanCommandLineTool(t *testing.T) { TestingT(t) }

type CmdSuite struct {
	backend    Backend
	out        *bytes.Buffer
	cmd        *Command
	testServer *httptest.Server
}

var _ = Suite(&CmdSuite{})

func (s *CmdSuite) SetUpSuite(c *C) {
	log.Init([]*log.LogConfig{&log.LogConfig{Name: "console"}})
}

func (s *CmdSuite) SetUpTest(c *C) {
	s.backend = membackend.NewMemBackend(registry.GetRegistry())

	muxRouter := mux.NewRouter()
	hostRouter := hostroute.NewHostRouter()
	proxy, err := vulcan.NewProxy(hostRouter)
	configurator := configure.NewConfigurator(proxy)
	c.Assert(err, IsNil)

	api.InitProxyController(s.backend, adapter.NewAdapter(proxy), configurator.GetConnWatcher(), muxRouter)
	s.testServer = httptest.NewServer(muxRouter)

	s.out = &bytes.Buffer{}
	s.cmd = &Command{registry: registry.GetRegistry(), out: s.out, vulcanUrl: s.testServer.URL}
}

func (s *CmdSuite) runString(in string) string {
	return s.run(strings.Split(in, " ")...)
}

func (s *CmdSuite) run(params ...string) string {
	args := []string{"vulcanctl"}
	args = append(args, params...)
	args = append(args, fmt.Sprintf("--vulcan=%s", s.testServer.URL))
	s.cmd.Run(args)
	return strings.Replace(s.out.String(), "\n", " ", -1)
}

func (s *CmdSuite) TestStatus(c *C) {
	c.Assert(s.run("status"), Matches, ".*hosts.*")
}

func (s *CmdSuite) TestHostCrud(c *C) {
	host := "host"
	c.Assert(s.run("host", "add", "-name", host), Matches, ".*added.*")
	c.Assert(s.run("host", "rm", "-name", host), Matches, ".*deleted.*")
}

func (s *CmdSuite) TestUpstreamCrud(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "ls"), Matches, fmt.Sprintf(".*%s.*", up))
}

func (s *CmdSuite) TestUpstreamAutoId(c *C) {
	c.Assert(s.run("upstream", "add"), Matches, ".*added.*")
}

func (s *CmdSuite) TestEndpointCrud(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	e := "e"
	c.Assert(s.run("endpoint", "add", "-id", e, "-url", "http://localhost:5000", "-up", up), Matches, ".*added.*")

	c.Assert(s.run("endpoint", "rm", "-id", e, "-up", up), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
}

func (s *CmdSuite) TestLimitsCrud(c *C) {
	// Create upstream with this location
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, ".*added.*")

	loc := "loc"
	path := "/path"
	c.Assert(s.run("location", "add", "-host", h, "-id", loc, "-up", up, "-path", path), Matches, ".*added.*")

	rl := "rl"
	c.Assert(s.run("ratelimit", "add", "-host", h, "-loc", loc, "-id", rl, "-requests", "10", "-variable", "client.ip", "-period", "3"), Matches, ".*added.*")
	c.Assert(s.run("ratelimit", "update", "-host", h, "-loc", loc, "-id", rl, "-requests", "100", "-variable", "client.ip", "-period", "30"), Matches, ".*updated.*")
	c.Assert(s.run("ratelimit", "rm", "-host", h, "-loc", loc, "-id", rl), Matches, ".*deleted.*")

	cl := "cl"
	c.Assert(s.run("connlimit", "add", "-host", h, "-loc", loc, "-id", cl, "-connections", "10", "-variable", "client.ip"), Matches, ".*added.*")
	c.Assert(s.run("connlimit", "update", "-host", h, "-loc", loc, "-id", cl, "-connections", "100", "-variable", "client.ip"), Matches, ".*updated.*")
	c.Assert(s.run("connlimit", "rm", "-host", h, "-loc", loc, "-id", cl), Matches, ".*deleted.*")

	c.Assert(s.run("location", "rm", "-host", h, "-id", loc), Matches, ".*deleted.*")
	c.Assert(s.run("host", "rm", "-name", h), Matches, ".*deleted.*")
	c.Assert(s.run("upstream", "rm", "-id", up), Matches, ".*deleted.*")
}

func (s *CmdSuite) TestUpstreamDrainConnections(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")
	c.Assert(s.run("upstream", "drain", "--id", up, "--timeout", "0"), Matches, ".*Connections: 0.*")
}

func (s *CmdSuite) TestLocationOptions(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, ".*added.*")

	loc := "loc"
	path := "/path"
	c.Assert(s.run(
		"location", "add",
		"-host", h, "-id", loc, "-up", up, "-path", path,
		// Timeouts
		"-readTimeout", "1s", "-dialTimeout", "2s", "-handshakeTimeout", "3s",
		// Keep Alive parameters
		"-keepAlivePeriod", "4s", "-maxIdleConns", "5",
		// Limits
		"-maxMemBodyKB", "6", "-maxBodyKB", "7",
		// Misc parameters
		// Failover predicate
		"-failoverPredicate", "IsNetworkError",
		// Forward header
		"-trustForwardHeader",
		// Forward host
		"-forwardHost", "host1",
	),
		Matches, ".*added.*")

	l, err := s.backend.GetLocation(h, loc)
	c.Assert(err, IsNil)
	c.Assert(l.Options.Timeouts.Read, Equals, "1s")
	c.Assert(l.Options.Timeouts.Dial, Equals, "2s")
	c.Assert(l.Options.Timeouts.TlsHandshake, Equals, "3s")

	c.Assert(l.Options.KeepAlive.Period, Equals, "4s")
	c.Assert(l.Options.KeepAlive.MaxIdleConnsPerHost, Equals, 5)

	c.Assert(l.Options.Limits.MaxMemBodyBytes, Equals, int64(6*1024))
	c.Assert(l.Options.Limits.MaxBodyBytes, Equals, int64(7*1024))

	c.Assert(l.Options.FailoverPredicate, Equals, "IsNetworkError")
	c.Assert(l.Options.TrustForwardHeader, Equals, true)
	c.Assert(l.Options.Hostname, Equals, "host1")
}

func (s *CmdSuite) TestLocationUpdateOptions(c *C) {
	up := "up"
	c.Assert(s.run("upstream", "add", "-id", up), Matches, ".*added.*")

	h := "h"
	c.Assert(s.run("host", "add", "-name", h), Matches, ".*added.*")

	loc := "loc"
	path := "/path"
	c.Assert(s.run("location", "add", "-host", h, "-id", loc, "-up", up, "-path", path), Matches, ".*added.*")
	s.run("location", "set_options", "-host", h, "-id", loc, "-dialTimeout", "20s")

	l, err := s.backend.GetLocation(h, loc)
	c.Assert(err, IsNil)
	c.Assert(l.Options.Timeouts.Dial, Equals, "20s")
}
