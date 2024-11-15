package bot

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/cankill/parental-control/internal/tools"
	tele "gopkg.in/telebot.v4"
)

func StartBot(requests chan tools.Request) {
	fmt.Println("Running bot")
	index := 0
	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// b.Use(middleware.Logger())
	// b.Use(middleware.AutoRespond())

	// Command: /start <PAYLOAD>
	b.Handle("/youtube", func(c tele.Context) error {
		fmt.Println(c.Message().Payload) // <PAYLOAD>
		return c.Send("Ok")
	})

	b.Handle("/ping", func(c tele.Context) error {
		return c.Send("pong")
	})

	b.Handle("/status", func(c tele.Context) error {
		responseChan := make(chan []tools.AppInfo)
		requests <- tools.Request{ResponseChan: responseChan}
		appInfos := <-responseChan
		var statistics string
		for _, appInfo := range appInfos {
			statistics += appInfo.Dump()
		}

		return c.Send(statistics)
	})

	// Command: /start <PAYLOAD>
	b.Handle("/screen", func(c tele.Context) error {
		fname := fmt.Sprintf("/tmp/pc/capture-%d.png", index)
		cmd := exec.Command("/usr/sbin/screencapture", fname)
		if err := cmd.Run(); err != nil {
			fmt.Println("Error: ", err)
			return c.Send(fmt.Sprintf("Error : %s", err))
		}

		image := &tele.Photo{File: tele.FromDisk(fname)}

		return c.Send(image)
	})

	b.Start()
}
