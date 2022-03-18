package platforms

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/carlmontanari/boxen/boxen/instance"
	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/util"

	scrapliocnos "github.com/hellt/scrapligo-ocnos/ocnos"

	"github.com/scrapli/scrapligo/cfg"

	"github.com/scrapli/scrapligo/driver/base"
	"github.com/scrapli/scrapligo/driver/core"
	"github.com/scrapli/scrapligo/driver/network"
)

const (
	defaultCommsReturnChar  = "\r\n"
	defaultConsoleTimeout   = 120
	defaultConsoleSleep     = 2
	defaultMaxLoginAttempts = 50
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
	options ...base.Option,
) (*ScrapliConsole, error) {
	opts := []base.Option{
		base.WithPort(port),
		base.WithAuthUsername(usr),
		base.WithAuthPassword(pwd),
		base.WithAuthSecondary(pwd),
		base.WithAuthBypass(true),
		base.WithCommsReturnChar(defaultCommsReturnChar),
		base.WithTransportType("telnet"),
		base.WithTimeoutTransport(0),
	}

	opts = append(opts, options...)

	if l.Console != nil {
		opts = append(opts, base.WithChannelLog(l.Console))
	}

	var c *network.Driver

	var err error

	switch scrapliPlatform {
	case IPInfusionOcNOSScrapliPlatform:
		c, err = scrapliocnos.NewOcNOSDriver(
			"localhost",
			opts...,
		)
	default:
		c, err = core.NewCoreDriver(
			"localhost",
			scrapliPlatform,
			opts...,
		)
	}

	if err != nil {
		return nil, err
	}

	con := &ScrapliConsole{
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
	_ = c.c.Transport.Close()

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

	ch := make(chan error)

	go func() {
		b := make([]byte, 0)

		for {
			cr, err := c.c.Channel.Read()
			if err != nil {
				ch <- err
			}

			b = append(b, bytes.ToLower(cr)...)

			if bytes.Contains(b, readUntil) {
				ch <- nil

				return
			}

			time.Sleep(1 * time.Second)
		}
	}()

	timeout = util.ApplyTimeoutMultiplier(timeout)

	timer := time.NewTimer(time.Duration(timeout) * time.Second)

	select {
	case <-ch:
		c.c.Channel.TimeoutOps = origChannelTimeoutOps
		return nil
	case <-timer.C:
		c.c.Channel.TimeoutOps = origChannelTimeoutOps
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

	tt := util.ApplyTimeoutMultiplier(10) // nolint:gomnd
	c.c.Transport.BaseTransportArgs.TimeoutTransport = time.Duration(tt) * time.Second

	loginAttempts := 0

	b := make([]byte, 0)

	for {
		//  in theory this could block forever due to the way scrapli handles timeouts -- if the
		//  read operation "times out" that goroutine does not actually die because it is still in
		//  a blocking read. there doesn't appear to be any good solution to this because you can't
		//  set read deadlines on os.File objects... the good news is that this seems to *not*
		//  actually cause any issues somehow (which honestly doesn't make a lot of sense), so we
		//  will let it slide for now and see if/when this causes an issue (it almost certainly
		//  won't in the context of boxen thankfully!)
		cr, err := c.c.Channel.Read()
		if err != nil {
			// only error we would get here is a timeout error (in theory), in which case we want
			// to send a return to "help" the console get its life together
			_ = c.c.Channel.SendReturn()

			continue
		}

		b = append(b, cr...)

		if loginArgs.promptPatterns.Match(b) {
			break
		}

		if loginArgs.loginPattern.Match(b) {
			_ = c.c.Channel.WriteAndReturn([]byte(loginArgs.username), false)
			b = make([]byte, 0)
		}

		if loginArgs.passwordPattern.Match(b) {
			_ = c.c.Channel.WriteAndReturn([]byte(loginArgs.password), true)
			b = make([]byte, 0)
		}

		loginAttempts++

		if loginAttempts > defaultMaxLoginAttempts {
			return fmt.Errorf("%w: console login failed", util.ErrConsoleError)
		}

		time.Sleep(1 * time.Second)
	}

	c.c.Transport.BaseTransportArgs.TimeoutTransport = 0

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

	cfgConn, err := cfg.NewCfgDriver(
		c.c,
		c.pT,
	)

	if err != nil {
		c.logger.Criticalf("failed creating scraplicfg driver: %s", err)

		return err
	}

	err = cfgConn.Prepare()
	if err != nil {
		c.logger.Criticalf("failed running prepare method of scrpalicfg driver: %s", err)

		return err
	}

	_, err = cfgConn.LoadConfig(
		string(b),
		replace,
	)
	if err != nil {
		c.logger.Criticalf("failed creating scraplicfg driver: %s", err)

		return err
	}

	_, err = cfgConn.CommitConfig()
	if err != nil {
		c.logger.Criticalf("failed committing device configuration: %s", err)

		return err
	}

	err = cfgConn.Cleanup()
	if err != nil {
		c.logger.Criticalf("failed running cleanup method of scrpalicfg driver: %s", err)

		return err
	}

	c.logger.Info("install config complete")

	return nil
}
