// Standalone trace tool: launches a gogram bot session with the SAME token
// as tgbotd (which requires tgbotd to be OFF to avoid double-auth), and
// prints every raw update the bot receives from MTProto. This tells us
// definitively whether the "skipped message" bug is (a) MTProto not
// delivering it, (b) tgbotd's UpdateBuffer swallowing it, or (c) the
// translator dropping it.
package main

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
)

func main() {
	token := os.Getenv("TGBOTD_TOKEN")
	if token == "" {
		token = os.Getenv("TGBOTD_BENCH_TOKEN")
	}
	if token == "" {
		fmt.Fprintln(os.Stderr, "TGBOTD_TOKEN env var is required")
		os.Exit(2)
	}
	cli, err := telegram.NewClient(telegram.ClientConfig{
		AppID:       2040,
		AppHash:     "b18441a1ff607e10a989891a5462e627",
		Session:     "./data/trace.session",
		SessionName: "trace",
		LogLevel:    telegram.LogWarn,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "NewClient:", err)
		os.Exit(1)
	}
	if err := cli.Connect(); err != nil {
		fmt.Fprintln(os.Stderr, "Connect:", err)
		os.Exit(1)
	}
	authed, _ := cli.IsAuthorized()
	if !authed {
		if err := cli.LoginBot(token); err != nil {
			fmt.Fprintln(os.Stderr, "LoginBot:", err)
			os.Exit(1)
		}
	}
	me := cli.Me()
	fmt.Printf("connected as @%s id=%d\n", me.Username, me.ID)

	cli.AddRawHandler(nil, func(u telegram.Update, c *telegram.Client) error {
		t := reflect.TypeOf(u).String()
		switch v := u.(type) {
		case *telegram.UpdateNewMessage:
			if m, ok := v.Message.(*telegram.MessageObj); ok {
				fmt.Printf("[%s] %s: out=%v id=%d msg=%q\n", time.Now().Format("15:04:05.000"), t, m.Out, m.ID, m.Message)
			} else {
				fmt.Printf("[%s] %s (non-MessageObj)\n", time.Now().Format("15:04:05.000"), t)
			}
		case *telegram.UpdateEditMessage:
			if m, ok := v.Message.(*telegram.MessageObj); ok {
				fmt.Printf("[%s] %s: out=%v id=%d msg=%q\n", time.Now().Format("15:04:05.000"), t, m.Out, m.ID, m.Message)
			}
		default:
			fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05.000"), t)
		}
		return nil
	})

	fmt.Println("watching updates for 5 minutes...")
	time.Sleep(5 * time.Minute)
}
