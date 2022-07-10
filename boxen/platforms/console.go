package platforms

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/scrapli/scrapligo/platform"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/util"

	"github.com/scrapli/scrapligocfg"

	"github.com/scrapli/scrapligo/driver/network"

	soptions "github.com/scrapli/scrapligo/driver/options"
	sutil "github.com/scrapli/scrapligo/util"
)

const (
	defaultCommsReturnChar          = "\r\n"
	defaultConsoleTimeout           = 120
	defaultConsoleSleep             = 2
	defaultMaxLoginAttempts         = 5
	defaultMaxConsecutiveEmptyReads = 10
)

type ScrapliConsole struct {
	pT        string
	c         *network.Driver
	defOnOpen func(d *network.Driver) error
	logger    *logging.Instance
}

func NewScrapliConsole(
	scrapliPlatform string,
	port int,
	usr, pwd string,
	l *instance.Loggers,
	options ...sutil.Option,
) (*ScrapliConsole, error) {
	opts := []sutil.Option{
		soptions.WithPort(port),
		soptions.WithAuthUsername(usr),
		soptions.WithAuthPassword(pwd),
		soptions.WithAuthSecondary(pwd),
		soptions.WithAuthBypass(),
		soptions.WithReturnChar(defaultCommsReturnChar),
		soptions.WithTransportType("telnet"),
	}

	opts = append(opts, options...)

	if l.Console != nil {
		opts = append(opts, soptions.WithChannelLog(l.Console))
	}

	var c *network.Driver

	var err error

	var p *platform.Platform

	p, err = platform.NewPlatform(
		scrapliPlatform,
		"localhost",
		opts...,
	)
	if err != nil {
		return nil, err
	}

	c, err = p.GetNetworkDriver()

	if err != nil {
		return nil, err
	}

	con := &ScrapliConsole{
		// (hellt) TODO: change this to a method call that returns platform type out of a scrapli definition
		// to support plugging in platform types from yml files referenced via URL/paths
		pT:        scrapliPlatform,
		c:         c,
		defOnOpen: c.OnOpen,
		logger:    l.Base,
	}

	c.OnOpen = func(d *network.Driver) error { return nil }

	return con, nil
}

func (c *ScrapliConsole) Config(lines []string) error {
	_, err := c.c.SendConfigs(lines)
	return err
}

func (c *ScrapliConsole) Detach() error {
	_ = c.c.Transport.Close(true)

	return nil
}

func (c *ScrapliConsole) openRetry() error {
	ch := make(chan error)

	go func() {
		for {
			err := c.c.Open()
			if err == nil {
				ch <- nil

				return
			}

			time.Sleep(defaultConsoleSleep * time.Second)
		}
	}()

	timeout := util.GetEnvIntOrDefault(
		"BOXEN_CONSOLE_TIMEOUT",
		defaultConsoleTimeout,
	)

	timeout = util.ApplyTimeoutMultiplier(timeout)

	timer := time.NewTimer(time.Duration(timeout) * time.Second)

	select {
	case r := <-ch:
		return r
	case <-timer.C:
		return fmt.Errorf("%w: failed opening console", util.ErrConsoleError)
	}
}

func (c *ScrapliConsole) readUntil(readUntil []byte, timeout int) error {
	readUntil = bytes.ToLower(readUntil)

	origChannelTimeoutOps := c.c.Channel.TimeoutOps
	c.c.Channel.TimeoutOps = 0

	defer func() {
		c.c.Channel.TimeoutOps = origChannelTimeoutOps
	}()

	ch := make(chan error)

	go func() {
		b := make([]byte, 0)

		for {
			cr, err := c.c.Channel.ReadAll()
			if err != nil {
				ch <- err
			}

			b = append(b, bytes.ToLower(cr)...)

			if bytes.Contains(b, readUntil) {
				ch <- nil

				return
			}
		}
	}()

	timeout = util.ApplyTimeoutMultiplier(timeout)

	timer := time.NewTimer(time.Duration(timeout) * time.Second)

	select {
	case <-ch:
		return nil
	case <-timer.C:
		return fmt.Errorf("%w: console read timeout", util.ErrConsoleError)
	}
}

type loginArgs struct {
	username        string
	password        string
	loginPattern    *regexp.Regexp
	passwordPattern *regexp.Regexp
	promptPatterns  *regexp.Regexp
}

// Convenience function to create a single "joined" pattern representing all possible prompt
// patterns for a scrapligo platform.
func (c *ScrapliConsole) joinScrapliPatterns() *regexp.Regexp {
	allPromptPatterns := make([]string, 0)

	for _, privLevel := range c.c.PrivilegeLevels {
		// yoink the flag statement and the line anchors off the patterns, we don't want them here
		allPromptPatterns = append(
			allPromptPatterns,
			fmt.Sprintf("(%s)", privLevel.Pattern[6:][:len(privLevel.Pattern)-7]),
		)
	}

	joinedPromptPatterns := `(?i)` + strings.Join(allPromptPatterns, "|")

	return regexp.MustCompile(joinedPromptPatterns)
}

func (c *ScrapliConsole) login(
	loginArgs *loginArgs,
) error {
	if loginArgs.loginPattern == nil {
		loginArgs.loginPattern = regexp.MustCompile(`(?im)^(.*username:)|(.*login:)\s?$`)
	}

	if loginArgs.passwordPattern == nil {
		loginArgs.passwordPattern = regexp.MustCompile(`(?im)^(.*@.*)?password:\s?$`)
	}

	if loginArgs.promptPatterns == nil {
		loginArgs.promptPatterns = c.joinScrapliPatterns()
	}

	consecutiveEmptyReads := 0
	maxConsecutiveEmptyReads := util.ApplyTimeoutMultiplier(defaultMaxConsecutiveEmptyReads)

	loginAttempts := 0

	b := make([]byte, 0)

	for {
		cr, err := c.c.Channel.ReadAll()
		if err != nil {
			return err
		}

		if cr == nil {
			// consecutive empty reads may mean we need to send a "return" character to help things
			// along...
			consecutiveEmptyReads++
		}

		if consecutiveEmptyReads == maxConsecutiveEmptyReads {
			c.logger.Debug("encountered too many consecutive empty reads, sending return...")

			consecutiveEmptyReads = 0

			_ = c.c.Channel.WriteReturn()

			continue
		}

		b = append(b, cr...)

		if loginArgs.promptPatterns.Match(b) {
			c.logger.Info("found device prompt, done handling login")

			break
		}

		if loginArgs.loginPattern.Match(b) {
			loginAttempts++

			c.logger.Debugf("found login prompt sending username %s", loginArgs.username)

			_ = c.c.Channel.WriteAndReturn([]byte(loginArgs.username), false)
			b = make([]byte, 0)
		}

		if loginArgs.passwordPattern.Match(b) {
			c.logger.Debugf("found password prompt sending password %s", loginArgs.password)

			_ = c.c.Channel.WriteAndReturn([]byte(loginArgs.password), true)
			b = make([]byte, 0)
		}

		if loginAttempts > defaultMaxLoginAttempts {
			return fmt.Errorf("%w: console login failed", util.ErrConsoleError)
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func (c *ScrapliConsole) InstallConfig(f string, replace bool) error {
	c.logger.Info("install config requested")

	resolvedF, err := util.ResolveFile(f)
	if err != nil {
		c.logger.Criticalf(
			"failed resolving provided config file '%s', error: %s",
			f,
			err,
		)

		return err
	}

	b, err := os.ReadFile(resolvedF)
	if err != nil {
		c.logger.Criticalf(
			"failed failed loading config from file '%s', error: %s",
			resolvedF,
			err,
		)

		return err
	}

	cfgConn, err := scrapligocfg.NewCfg(
		c.c,
		c.pT,
	)

	if err != nil {
		c.logger.Criticalf("failed creating scrapli cfg driver: %s", err)

		return err
	}

	err = cfgConn.Prepare()
	if err != nil {
		c.logger.Criticalf("failed running prepare method of scrapli cfg driver: %s", err)

		return err
	}

	_, err = cfgConn.LoadConfig(
		string(b),
		replace,
	)
	if err != nil {
		c.logger.Criticalf("failed creating scrapli cfg driver: %s", err)

		return err
	}

	_, err = cfgConn.CommitConfig()
	if err != nil {
		c.logger.Criticalf("failed committing device configuration: %s", err)

		return err
	}

	err = cfgConn.Cleanup()
	if err != nil {
		c.logger.Criticalf("failed running cleanup method of scrapli cfg driver: %s", err)

		return err
	}

	c.logger.Info("install config complete")

	return nil
}
