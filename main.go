package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/kapitanov/go-cube"
	"github.com/mxk/go-imap/imap"
	"github.com/ttacon/chalk"
)

var trace = log.New(os.Stdout, "[cube-mail]", log.Ltime)
var configFileName string
var verboseLog bool

func init() {
	flag.StringVar(&configFileName, "config", "cube-gmail.json", "Path to config file")
	flag.BoolVar(&verboseLog, "v", false, "Enable verbose logging")

	imap.DefaultLogger = log.New(os.Stdout, "[imap]", log.Ltime)
}

var logoStyle = chalk.Cyan.NewStyle().Style

func main() {
	fmt.Println(":: ", logoStyle("IMAP monitor for Amperka Cube"), " ::\n\n")
	flag.Parse()

	config, err := LoadConfig(configFileName)
	if err != nil {
		panic(err)
	}

	if !verboseLog {
		w := io.MultiWriter()
		trace.SetOutput(w)
		imap.DefaultLogger.SetOutput(w)
		cube.SetLogWriter(w)
	}

	RunMonitor(config)
}
