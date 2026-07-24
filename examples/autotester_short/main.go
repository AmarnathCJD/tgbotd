package main
import ("fmt"; "os"; "strings"; "sync"; "sync/atomic"; "time"
	"github.com/amarnathcjd/gogram/telegram")
// Set TGBOTD_STRING_SESSION to a gogram StringSession for a real Telegram user
// account (see examples/README.md). Optional TGBOTD_TARGET_BOT overrides the
// default target username.
type rec struct { sent, seen time.Time; sentTxt, replyTxt string }
func main() {
	session := os.Getenv("TGBOTD_STRING_SESSION")
	if session == "" {
		fmt.Println("TGBOTD_STRING_SESSION env var is required")
		os.Exit(2)
	}
	target := os.Getenv("TGBOTD_TARGET_BOT")
	if target == "" {
		target = "Waw283Bot"
	}
	cli, _ := telegram.NewClient(telegram.ClientConfig{AppID:2040, AppHash:"b18441a1ff607e10a989891a5462e627", StringSession:session, LogLevel:telegram.LogWarn})
	cli.Connect()
	if authed,_ := cli.IsAuthorized(); !authed { fmt.Println("not authed"); os.Exit(1) }
	me := cli.Me()
	fmt.Printf("[autotester] user=%d @%s\n", me.ID, me.Username)
	bot, err := cli.ResolveUsername(target)
	if err != nil { fmt.Println(err); os.Exit(1) }
	var botID int64
	if v, ok := bot.(*telegram.UserObj); ok { botID = v.ID } else { os.Exit(1) }

	var records []*rec
	var mu sync.Mutex
	var seen atomic.Int32
	cli.AddMessageHandler(telegram.OnMessage, func(m *telegram.NewMessage) error {
		if m == nil || m.Message == nil { return nil }
		if fromID, ok := m.Message.FromID.(*telegram.PeerUser); ok && fromID.UserID == botID {
			mu.Lock()
			for _, r := range records {
				if r.seen.IsZero() {
					r.seen = time.Now(); r.replyTxt = m.Message.Message
					seen.Add(1)
					fmt.Printf("[reply] %q\n", strings.ReplaceAll(m.Message.Message,"\n","\n"))
					break
				}
			}
			mu.Unlock()
		}
		return nil
	})
	go cli.Idle()
	time.Sleep(1500 * time.Millisecond)

	fmt.Println("\n=== 4 pings, 500ms spacing ===")
	for i := 0; i < 4; i++ {
		txt := fmt.Sprintf("/ping short-%d", i)
		r := &rec{sent: time.Now(), sentTxt: txt}
		mu.Lock(); records = append(records, r); mu.Unlock()
		cli.SendMessage(botID, txt)
		time.Sleep(500 * time.Millisecond)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && seen.Load() < int32(len(records)) { time.Sleep(200 * time.Millisecond) }

	mu.Lock()
	ok := 0
	for _, r := range records {
		if !r.seen.IsZero() { ok++ }
	}
	fmt.Printf("\n=== RESULT: %d/%d ===\n", ok, len(records))
	for _, r := range records {
		st := "MISS"
		if !r.seen.IsZero() { st = "OK" }
		fmt.Printf("  [%s] %s\n", st, r.sentTxt)
	}
	mu.Unlock()
}
