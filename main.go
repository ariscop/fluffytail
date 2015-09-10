package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"strconv"
	"time"
	"encoding/json"

	"github.com/shizeeg/gcfg"
	"github.com/thoj/go-ircevent"
)

type Config struct {
	IRC struct {
		Host     string // IRC address to connect to (ip:port)
		Password string // password (if any)
		UseSSL   bool   // connect via SSL?
		Channel  string // channel to output logs to
	}
	Bot struct {
		Nick      string   // bot nick
		User      string   // bot user ("ident")
		OnConnect []string // commands (if any) to send on connect.
		SendDelay int      // milliseconds to wait between messages
	}
}

var (
	configLocation = flag.String("conf", "./fluffytail.conf", "config file to load and use")
	config         = &Config{}
)

func main() {
	flag.Parse()

	log.Printf("reading config file %s\n", *configLocation)
	gcfg.ReadFileInto(config, *configLocation)

	log.Printf("creating bot %s!%s@*", config.Bot.Nick, config.Bot.User)
	bot := irc.IRC(config.Bot.Nick, config.Bot.User)
	bot.UseTLS = config.IRC.UseSSL
	bot.Password = config.IRC.Password

	log.Printf("connecting bot to %s", config.IRC.Host)
	bot.Connect(config.IRC.Host)

	queue := make(chan string, 0)

	bot.AddCallback("001", func(e *irc.Event) {
		for _, cmd := range config.Bot.OnConnect {
			log.Printf("sending raw line '%s'", cmd)

			bot.SendRaw(cmd)
		}

		log.Printf("joining %s", config.IRC.Channel)
		bot.Join(config.IRC.Channel)

		go watchLogs(bot, queue)
	})

	bot.AddCallback("PRIVMSG", func(e *irc.Event) {
		if strings.HasPrefix(e.Message(), "!sys-stats") {
			subProcess := exec.Command("/usr/bin/uptime")
			readOutputFromCommand(subProcess, queue)

			subProcess = exec.Command("/usr/bin/free", "-m")
			readOutputFromCommand(subProcess, queue)
		}
	})

	bot.Loop()
}

func watchLogs(bot *irc.Connection, queue chan string) {
	subProcess := exec.Command("/usr/bin/journalctl", "-f", "-o", "json")
	go readOutputFromCommand(subProcess, queue)

	for msg := range queue {
		time.Sleep(time.Duration(config.Bot.SendDelay) * time.Millisecond)
		bot.Privmsg(config.IRC.Channel, msg)
	}

}

func readOutputFromCommand(subProcess *exec.Cmd, queue chan string) {
	stdout, err := subProcess.StdoutPipe()
	if err != nil {
		log.Fatal("failed to call subprocess: %v", err)
	}

	err = subProcess.Start()
	if err != nil {
		log.Fatal("failed to start: %v", err)
	}

	scanner := bufio.NewScanner(stdout)

	for scanner.Scan() {
		line := scanner.Text()

		if scanner.Err() != nil {
			log.Fatalf("failed reading a line somewhere (got '%s') -- dying", line)
		}

		var record map[string]string
		err = json.Unmarshal([]byte(line), &record)
		if err != nil {
			log.Fatalf("failed to unmarshal json (got '%s') -- dying", line)
		}

		queue <- formatRecord(record)
	}
}

func formatRecord(record map[string]string) string {
	unit := getUnitName(record)

	priority, err := strconv.Atoi(record["PRIORITY"])
	if err != nil {
		// Default to info, many records lack PRIORITY
		priority = 6
	}

	colour := ""

	if priority <= 5 { // Notice or higher, use bold
		colour = "\x02"
	}
	if priority <= 3 { // err and higher, use bold red
		colour += "\x0304"
	}

	return fmt.Sprintf("\x02%s[%s]:\x0f%s %s", unit, record["_PID"], colour, record["MESSAGE"]);
}


func getUnitName(record map[string]string) string {
	unit, ok := record["_SYSTEMD_UNIT"]

	if strings.HasSuffix(unit, ".scope") {
		// Scopes aren't useful to see, don't use them
		ok = false
	}
	if !ok {
		// Use _COMM (executable name) when unit name is missing
		unit, ok = record["_COMM"]
	}
	if !ok {
		// Maybe it's from syslog?
		unit, ok = record["SYSLOG_IDENTIFIER"]
	}
	if !ok {
		// For something lacking all the previous, it's probably over
		// some specific transport
		// eg: kernel log, Audit log
		unit = record["_TRANSPORT"]
	}

	// Strip .service suffix if present, useless noise when almost
	// everything is a service
	return strings.TrimSuffix(unit, ".service")
}
