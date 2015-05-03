package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"time"

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

	subProcess := exec.Command("/usr/bin/journalctl", "-f", "-l")

	stdout, err := subProcess.StdoutPipe()
	if err != nil {
		log.Fatal("failed to get stdout pipe for log gathering subprocess")
	}

	err = subProcess.Start()
	if err != nil {
		log.Fatal("failed to start log gathering subprocess: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	msgs := make(chan string, 0)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()

			if scanner.Err() != nil {
				log.Fatalf("failed reading a line somewhere (got '%s') -- dying", line)
			}

			msgs <- fmt.Sprintf("<-- %s", line)
		}
	}()

	go func() {
		for msg := range msgs {
			time.Sleep(time.Duration(config.Bot.SendDelay) * time.Millisecond)
			bot.Privmsg(config.IRC.Channel, msg)
		}
	}()

	bot.AddCallback("001", func(e *irc.Event) {
		for _, cmd := range config.Bot.OnConnect {
			log.Printf("sending raw line '%s'", cmd)

			bot.SendRaw(cmd)
		}

		log.Printf("joining %s", config.IRC.Channel)
		bot.Join(config.IRC.Channel)
	})

	bot.Loop()
}
