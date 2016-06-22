package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kapitanov/go-cube"
	"github.com/mxk/go-imap/imap"

	"github.com/ttacon/chalk"
)

var imapLogMask = imap.LogConn | imap.LogState | imap.LogCmd | imap.LogRaw
var imapSafeLogMask = imap.LogConn | imap.LogState

var numberStyle = chalk.Cyan.NewStyle().Style
var offStyle = chalk.Black.NewStyle().WithBackground(chalk.White).Style
var greenStyle = chalk.White.NewStyle().WithBackground(chalk.Green).Style
var redStyle = chalk.White.NewStyle().WithBackground(chalk.Red).Style

func RunMonitor(config *Config) {
	imap.DefaultLogMask = imapLogMask

	monitor := NewMboxMonitor(config)
	monitor.Run()
}

const loopSleepPeriod = 1 * time.Second
const errorSleepPeriod = 60 * time.Second

type MboxMonitor struct {
	config    *Config
	client    *imap.Client
	driver    *CubeDriver
	prevCount uint32
}

func NewMboxMonitor(config *Config) *MboxMonitor {
	mon := new(MboxMonitor)
	mon.config = config
	return mon
}

func (m *MboxMonitor) Run() {

	termReq := make(chan os.Signal, 1)
	timer := make(chan int)

	signal.Notify(termReq, os.Interrupt)
	signal.Notify(termReq, syscall.SIGTERM)

	m.driver = NewCubeDriver(m.config)
	go m.driver.Run()
	for {
		m.RunOnce()

		go func() {
			time.Sleep(loopSleepPeriod)
			timer <- 0
		}()

		select {
		case <-termReq:
			m.driver.Close()
			m.Disconnect()
			fmt.Println("Goodbye!")
			os.Exit(1)
			return
		case <-timer:
		}
	}
}

func (m *MboxMonitor) RunOnce() {
	if m.client == nil {
		fmt.Printf("Connecting to %s...\n", m.config.Addr)
		err := m.Connect()
		if err != nil {
			fmt.Printf("ERROR! %s\nWill now sleep for %s\n", err, errorSleepPeriod)
			m.Disconnect()
			time.Sleep(errorSleepPeriod)
			return
		} else {
			fmt.Printf("Connected!\n")
		}
	}

	count, err := m.QueryUnreadCount()
	if err != nil {
		fmt.Printf("ERROR! %s\nWill now sleep for %s\n", err, errorSleepPeriod)
		m.Disconnect()
		time.Sleep(errorSleepPeriod)
		return
	}

	if m.prevCount != count {
		m.prevCount = count
		fmt.Println("You've got ", numberStyle(fmt.Sprintf("%d", count)), "unread message(s)!")
	}

	if count > m.config.RedIfMore {
		m.driver.Set(RED)
		return
	}

	if count > m.config.GreenIfMore {
		m.driver.Set(GREEN)
		return
	}

	m.driver.Set(OFF)
}

func (m *MboxMonitor) Connect() error {
	var c *imap.Client
	var err error

	addr := m.config.Addr
	if strings.HasSuffix(addr, ":993") {
		c, err = imap.DialTLS(addr, nil)
	} else {
		c, err = imap.Dial(addr)
	}

	m.client = c

	if err != nil {
		fmt.Printf("ERROR! Unable to dial '%s': %s\n", addr, err)
		return err
	}

	if c.Caps["STARTTLS"] {
		_, err = check(m.client.StartTLS(nil))
		if err != nil {
			fmt.Printf("ERROR! Unable to start TLS: %s\n", err)
			return err
		}
	}

	c.SetLogMask(imapSafeLogMask)
	_, err = check(m.client.Login(m.config.Username, m.config.Password))
	c.SetLogMask(imapLogMask)
	if err != nil {
		fmt.Printf("ERROR! Unable to login: %s\n", err)
		return err
	}

	return nil
}

func (m *MboxMonitor) Disconnect() {
	m.driver.Set(OFF)
	if m.client != nil {
		m.client.Close(false)
	}
	m.client = nil
}

func (m *MboxMonitor) QueryUnreadCount() (uint32, error) {
	cmd, err := check(m.client.Status(m.config.Label))
	if err != nil {
		return 0, err
	}

	var count uint32
	for _, result := range cmd.Data {
		mailboxStatus := result.MailboxStatus()
		if mailboxStatus != nil {
			count += mailboxStatus.Unseen
		}
	}

	return count, nil
}

func check(cmd *imap.Command, err error) (*imap.Command, error) {
	if err != nil {
		trace.Printf("IMAP ERROR: %s", err)
		return nil, err
	}

	_, err = cmd.Result(imap.OK)
	if err != nil {
		trace.Printf("COMMAND ERROR: %s", err)
		return nil, err
	}

	return cmd, err
}

type CubeDriver struct {
	config      *Config
	mutex       sync.Mutex
	state       CubeState
	c           cube.Cube
	terminating bool
	terminated  chan int
}

type CubeState int

const (
	OFF       CubeState = 0
	GREEN     CubeState = 1
	RED       CubeState = 2
	TERMINATE CubeState = 1000
)

func NewCubeDriver(config *Config) *CubeDriver {
	drv := new(CubeDriver)
	drv.config = config
	drv.terminated = make(chan int)
	return drv
}

const waitPeriod = 10 * time.Second
const blinkPeriod = 100 * time.Millisecond

func (c *CubeDriver) Set(state CubeState) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.state != state {
		c.state = state

		switch state {
		case RED:
			fmt.Println("Cube is now ", redStyle("RED"))
		case GREEN:
			fmt.Println("Cube is now ", greenStyle("GREEN"))
		case OFF:
			fmt.Println("Cube is now ", offStyle("OFF"))
		}
	}
}

func (c *CubeDriver) Close() {
	c.Set(OFF)
	c.mutex.Lock()
	c.terminating = true
	c.mutex.Unlock()
	<-c.terminated
}

func (c *CubeDriver) Run() {
	for {
		if c.c == nil {
			drv, err := cube.NewCube(c.config.Cube)
			if err != nil {
				time.Sleep(waitPeriod)
				continue
			}

			c.c = drv
		}

		c.mutex.Lock()
		state := c.state
		terminating := c.terminating
		c.mutex.Unlock()

		if terminating {
			c.c.Off()
			c.c.Close()
			c.terminated <- 0
			return
		}

		switch state {
		case GREEN:
			c.c.Green()
			time.Sleep(blinkPeriod)
			c.c.Off()
			time.Sleep(blinkPeriod)

		case RED:
			c.c.Red()
			time.Sleep(blinkPeriod)
			c.c.Off()
			time.Sleep(blinkPeriod)

		default:
			c.c.Off()
			time.Sleep(blinkPeriod)
		}
	}
}
