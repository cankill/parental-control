package bot

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"parental-control/internal/lib/types"
	"slices"
	"time"

	"github.com/olekukonko/tablewriter"
	tele "gopkg.in/telebot.v4"
)

func StartBot(requests chan<- types.Request) {
	defer func() {
		fmt.Println("Bot finished")
	}()

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
		responseChan := make(chan []types.AppInfo)
		requests <- types.Request{ResponseChan: responseChan}
		appInfos := <-responseChan

		slices.SortFunc(appInfos, func(a types.AppInfo, b types.AppInfo) int {
			if a.Duration < b.Duration {
				return 1
			}
			return -1
		})

		var buf bytes.Buffer
		table := tablewriter.NewWriter(&buf)
		table.SetHeader([]string{"App", "Time spent"})
		table.SetBorder(false)
		total := time.Duration(0)
		for _, appInfo := range appInfos {
			table.Append(appInfo.Table())
			total += appInfo.Duration
		}
		table.SetFooter([]string{"Total", total.String()})
		table.Render()
		statistics := "```\n" + buf.String() + "\n```"

		return c.Send(statistics, &tele.SendOptions{
			ParseMode: tele.ModeMarkdownV2,
		})
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
