package server

import (
	"encoding/json"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/amarnathcjd/tgbotd/internal/botapi"
	"github.com/amarnathcjd/tgbotd/internal/botmgr"
	"github.com/amarnathcjd/tgbotd/internal/tlate"
)

func init() {
	register("sendinvoice", sendInvoice)
	register("createinvoicelink", createInvoiceLink)
	register("answershippingquery", answerShippingQuery)
	register("answerprecheckoutquery", answerPreCheckoutQuery)
	register("getmystarbalance", getMyStarBalance)
	register("getstartransactions", getStarTransactions)
	register("refundstarpayment", refundStarPayment)
	register("edituserstarsubscription", editUserStarSubscription)
}

// buildInvoiceMedia constructs a gogram InputMediaInvoice from Bot API params.
func buildInvoiceMedia(r *Request) (*telegram.InputMediaInvoice, error) {
	title, err := requireString(r, "title")
	if err != nil {
		return nil, err
	}
	desc, err := requireString(r, "description")
	if err != nil {
		return nil, err
	}
	payload, err := requireString(r, "payload")
	if err != nil {
		return nil, err
	}
	currency, err := requireString(r, "currency")
	if err != nil {
		return nil, err
	}
	pricesRaw, ok := paramRaw(r, "prices")
	if !ok {
		return nil, botapi.ErrBadRequest("field \"prices\" is required")
	}
	var priceObjs []struct {
		Label  string `json:"label"`
		Amount int64  `json:"amount"`
	}
	if err := json.Unmarshal(pricesRaw, &priceObjs); err != nil {
		return nil, botapi.ErrBadRequest("prices must be an array")
	}
	prices := make([]*telegram.LabeledPrice, len(priceObjs))
	for i, p := range priceObjs {
		prices[i] = &telegram.LabeledPrice{Label: p.Label, Amount: p.Amount}
	}
	inv := &telegram.Invoice{
		Currency: currency,
		Prices:   prices,
	}
	if v, ok := paramBool(r, "need_name"); ok && v {
		inv.NameRequested = true
	}
	if v, ok := paramBool(r, "need_phone_number"); ok && v {
		inv.PhoneRequested = true
	}
	if v, ok := paramBool(r, "need_email"); ok && v {
		inv.EmailRequested = true
	}
	if v, ok := paramBool(r, "need_shipping_address"); ok && v {
		inv.ShippingAddressRequested = true
	}
	if v, ok := paramBool(r, "is_flexible"); ok && v {
		inv.Flexible = true
	}
	if v, ok := paramBool(r, "send_phone_number_to_provider"); ok && v {
		inv.PhoneToProvider = true
	}
	if v, ok := paramBool(r, "send_email_to_provider"); ok && v {
		inv.EmailToProvider = true
	}
	if tip, ok := paramInt64(r, "max_tip_amount"); ok {
		inv.MaxTipAmount = tip
	}
	if raw, ok := paramRaw(r, "suggested_tip_amounts"); ok && len(raw) > 0 {
		var tips []int64
		if err := json.Unmarshal(raw, &tips); err == nil {
			inv.SuggestedTipAmounts = tips
		}
	}
	if sub, ok := paramInt64(r, "subscription_period"); ok {
		inv.SubscriptionPeriod = int32(sub)
	}
	provider, _ := paramString(r, "provider_token")
	providerData, _ := paramString(r, "provider_data")
	startParam, _ := paramString(r, "start_parameter")
	media := &telegram.InputMediaInvoice{
		Title:       title,
		Description: desc,
		Invoice:     inv,
		Payload:     []byte(payload),
		Provider:    provider,
		ProviderData: &telegram.DataJson{Data: providerData},
		StartParam:  startParam,
	}
	if photoURL, ok := paramString(r, "photo_url"); ok {
		wd := &telegram.InputWebDocument{
			URL:      photoURL,
			MimeType: "image/jpeg",
		}
		if sz, ok := paramInt64(r, "photo_size"); ok {
			wd.Size = int32(sz)
		}
		media.Photo = wd
	}
	return media, nil
}

func sendInvoice(s *Server, r *Request) (any, error) {
	peer, err := resolveChatID(r, "chat_id")
	if err != nil {
		return nil, err
	}
	media, err := buildInvoiceMedia(r)
	if err != nil {
		return nil, err
	}
	opts := commonMediaOpts(r)
	nm, err := r.Bot.Client.SendMedia(peer, media, opts)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return tlate.MessageObjToBotAPICtx(newMessageToObj(nm), r.Bot.BuildTranslateContext()), nil
}

func createInvoiceLink(s *Server, r *Request) (any, error) {
	media, err := buildInvoiceMedia(r)
	if err != nil {
		return nil, err
	}
	res, err := r.Bot.Client.PaymentsExportInvoice(media)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	if res == nil {
		return nil, botapi.Errorf(500, "empty invoice link result")
	}
	return res.URL, nil
}

func answerShippingQuery(s *Server, r *Request) (any, error) {
	qidStr, err := requireString(r, "shipping_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	if _, err := jsonParseInt(qidStr, &qid); err != nil {
		return nil, botapi.ErrBadRequest("shipping_query_id must be numeric")
	}
	ok, _ := paramBool(r, "ok")
	errText, _ := paramString(r, "error_message")
	var opts []*telegram.ShippingOption
	if ok {
		if raw, ok2 := paramRaw(r, "shipping_options"); ok2 {
			var arr []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Prices []struct {
					Label  string `json:"label"`
					Amount int64  `json:"amount"`
				} `json:"prices"`
			}
			if err := json.Unmarshal(raw, &arr); err == nil {
				for _, so := range arr {
					prices := make([]*telegram.LabeledPrice, len(so.Prices))
					for i, p := range so.Prices {
						prices[i] = &telegram.LabeledPrice{Label: p.Label, Amount: p.Amount}
					}
					opts = append(opts, &telegram.ShippingOption{ID: so.ID, Title: so.Title, Prices: prices})
				}
			}
		}
		errText = ""
	}
	if _, err := r.Bot.Client.MessagesSetBotShippingResults(qid, errText, opts); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func answerPreCheckoutQuery(s *Server, r *Request) (any, error) {
	qidStr, err := requireString(r, "pre_checkout_query_id")
	if err != nil {
		return nil, err
	}
	var qid int64
	if _, err := jsonParseInt(qidStr, &qid); err != nil {
		return nil, botapi.ErrBadRequest("pre_checkout_query_id must be numeric")
	}
	ok, _ := paramBool(r, "ok")
	errText, _ := paramString(r, "error_message")
	if _, err := r.Bot.Client.MessagesSetBotPrecheckoutResults(ok, qid, errText); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func getMyStarBalance(s *Server, r *Request) (any, error) {
	me := r.Bot.Client.Me()
	if me == nil {
		return nil, botapi.Errorf(500, "no cached self user")
	}
	stats, err := r.Bot.Client.PaymentsGetStarsRevenueStats(false, false, &telegram.InputPeerSelf{})
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	amount := int64(0)
	if stats != nil && stats.Status != nil {
		if a, ok := stats.Status.CurrentBalance.(*telegram.StarsAmountObj); ok {
			amount = a.Amount
		}
	}
	return map[string]any{"amount": amount}, nil
}

func getStarTransactions(s *Server, r *Request) (any, error) {
	// Placeholder: gogram exposes PaymentsGetStarsTransactions but the
	// signature is heavy — expose a minimal empty response for now.
	_ = r
	return map[string]any{"transactions": []any{}}, nil
}

func refundStarPayment(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	chargeID, err := requireString(r, "telegram_payment_charge_id")
	if err != nil {
		return nil, err
	}
	peer, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := peer.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must resolve to a user")
	}
	if _, err := r.Bot.Client.PaymentsRefundStarsCharge(&telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}, chargeID); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}

func editUserStarSubscription(s *Server, r *Request) (any, error) {
	uid, err := requireInt64(r, "user_id")
	if err != nil {
		return nil, err
	}
	chargeID, err := requireString(r, "telegram_payment_charge_id")
	if err != nil {
		return nil, err
	}
	isCanceled, _ := paramBool(r, "is_canceled")
	usr, err := r.Bot.Client.ResolvePeer(uid)
	if err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	pu, ok := usr.(*telegram.InputPeerUser)
	if !ok {
		return nil, botapi.ErrBadRequest("user_id must be a user")
	}
	if _, err := r.Bot.Client.PaymentsBotCancelStarsSubscription(!isCanceled, &telegram.InputUserObj{UserID: pu.UserID, AccessHash: pu.AccessHash}, chargeID); err != nil {
		return nil, botmgr.MapRPCError(err)
	}
	return true, nil
}
