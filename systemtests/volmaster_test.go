package systemtests

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/contiv/volplugin/config"
	. "gopkg.in/check.v1"
)

func (s *systemtestSuite) TestVolmasterNoGlobalConfiguration(c *C) {
	c.Assert(s.vagrant.IterateNodes(stopVolmaster), IsNil)
	_, err := s.mon0cmd("etcdctl rm /volplugin/global-config")
	c.Assert(err, IsNil)
	c.Assert(s.vagrant.IterateNodes(startVolmaster), IsNil)
	c.Assert(s.vagrant.IterateNodes(waitForVolmaster), IsNil)

	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)
	_, err = s.docker("run -v policy1/test:/mnt alpine echo")
	c.Assert(err, IsNil)
}

func (s *systemtestSuite) TestVolmasterFailedFormat(c *C) {
	_, err := s.uploadIntent("policy2", "fs")
	c.Assert(err, IsNil)
	c.Assert(s.createVolume("mon0", "policy2", "testfalse", map[string]string{"filesystem": "falsefs"}), NotNil)
	_, err = s.volcli("volume remove policy2/testfalse")
	c.Assert(err, NotNil)
}

func (s *systemtestSuite) TestVolmasterGlobalConfigUpdate(c *C) {
	content, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s/global1.json", getDriver()))
	c.Assert(err, IsNil)

	globalBase1, err := config.NewGlobalConfigFromJSON(content)
	c.Assert(err, IsNil)

	content, err = ioutil.ReadFile(fmt.Sprintf("testdata/%s/global2.json", getDriver()))
	c.Assert(err, IsNil)

	globalBase2, err := config.NewGlobalConfigFromJSON(content)
	c.Assert(err, IsNil)

	out, err := s.volcli("global get")
	c.Assert(err, IsNil)

	global, err := config.NewGlobalConfigFromJSON([]byte(out))
	c.Assert(err, IsNil)

	c.Assert(globalBase1, DeepEquals, global)
	c.Assert(globalBase2, Not(DeepEquals), global)

	c.Assert(s.uploadGlobal("global2"), IsNil)

	out, err = s.volcli("global get")
	c.Assert(err, IsNil)

	global, err = config.NewGlobalConfigFromJSON([]byte(out))
	c.Assert(err, IsNil)

	c.Assert(globalBase1, Not(DeepEquals), global)
	c.Assert(globalBase2, DeepEquals, global)
}

func (s *systemtestSuite) TestVolmasterMultiRemove(c *C) {
	c.Assert(s.createVolume("mon0", "policy1", "test", nil), IsNil)

	errChan := make(chan error, 5)
	outChan := make(chan string, 5)

	for i := 0; i < 5; i++ {
		go func() {
			out, err := s.volcli("volume remove policy1/test")
			outChan <- out
			errChan <- err
		}()
	}

	errs := 0

	for i := 0; i < 5; i++ {
		err := <-errChan
		out := <-outChan
		if err != nil {
			errs++
		}
		if out != "" {
			c.Assert(strings.Contains(out, `Removing volume policy1/test Volume policy1/test no longer exists`), Equals, true)
		}
	}

	c.Assert(errs, Equals, 4)
}
