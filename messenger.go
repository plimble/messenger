package messenger

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	// ProfileURL is the API endpoint used for retrieving profiles.
	// Used in the form: https://graph.facebook.com/v2.6/<USER_ID>?fields=first_name,last_name,profile_pic&access_token=<PAGE_ACCESS_TOKEN>
	ProfileURL = "https://graph.facebook.com/v2.6/"
)

type OnGetPageTokenFunc func(int64) (string, error)

// MessageHandler is a handler used for responding to a message containing text.
type OnMessageFunc func(*Messenger, Message, *Response)

// DeliveryHandler is a handler used for responding to a read receipt.
type OnDeliveryFunc func(*Messenger, Delivery, *Response)

// Messenger is the client which manages communication with the Messenger Platform API.
type Messenger struct {
	verifyToken        string
	onMessageFunc      OnMessageFunc
	onDeliveryFunc     OnDeliveryFunc
	onGetPageTokenFunc OnGetPageTokenFunc
}

// New creates a new Messenger. You pass in Options in order to affect settings.
func New(verifyToken string) *Messenger {
	m := &Messenger{
		verifyToken: verifyToken,
	}

	return m
}

// VerifyWebhook facebook will send GET method to verify the webhook with verify token
func (m *Messenger) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("hub.verify_token") == m.verifyToken {
		fmt.Fprintln(w, r.FormValue("hub.challenge"))
		return
	}

	fmt.Fprintln(w, "Incorrect verify token.")
}

func (m *Messenger) Webhook(w http.ResponseWriter, r *http.Request) {
	var rec Receive

	err := json.NewDecoder(r.Body).Decode(&rec)
	if err != nil {
		fmt.Println(err)

		fmt.Fprintln(w, `{status: 'not ok'}`)
		return
	}

	if rec.Object != "page" {
		fmt.Println("Object is not page, undefined behaviour. Got", rec.Object)
	}

	m.dispatch(rec, w)

	fmt.Fprintln(w, `{status: 'ok'}`)
}

func (m *Messenger) OnGetPageToken(f OnGetPageTokenFunc) {
	m.onGetPageTokenFunc = f
}

// HandleMessage adds a new MessageHandler to the Messenger which will be triggered
// when a message is received by the client.
func (m *Messenger) OnMessage(f OnMessageFunc) {
	m.onMessageFunc = f
}

// HandleDelivery adds a new DeliveryHandler to the Messenger which will be triggered
// when a previously sent message is read by the recipient.
func (m *Messenger) OnDelivery(f OnDeliveryFunc) {
	m.onDeliveryFunc = f
}

// ProfileByID retrieves the Facebook user associated with that ID
func (m *Messenger) ProfileByID(pageToken string, id int64) (Profile, error) {
	p := Profile{}
	url := fmt.Sprintf("%v%v", ProfileURL, id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return p, err
	}

	req.URL.RawQuery = "fields=first_name,last_name,profile_pic&access_token=" + pageToken

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&p)

	return p, err
}

// dispatch triggers all of the relevant handlers when a webhook event is received.
func (m *Messenger) dispatch(r Receive, w http.ResponseWriter) {
	for _, entry := range r.Entry {
		for _, info := range entry.Messaging {
			a := m.classify(info, entry)
			if a == UnknownAction {
				fmt.Println("Unknown action:", info)
				continue
			}

			pageToken, err := m.onGetPageTokenFunc(entry.ID)
			if err != nil {
				fmt.Println(err)
				fmt.Fprintln(w, `{status: 'not ok'}`)
				return
			}

			resp := &Response{
				to:    Recipient{info.Sender.ID},
				token: pageToken,
			}

			switch a {
			case TextAction:
				if m.onMessageFunc != nil {
					message := *info.Message
					message.Sender = info.Sender
					message.Recipient = info.Recipient
					message.Time = time.Unix(info.Timestamp, 0)
					message.PageToken = pageToken

					m.onMessageFunc(m, message, resp)
				}
			case DeliveryAction:
				if m.onDeliveryFunc != nil {
					m.onDeliveryFunc(m, *info.Delivery, resp)
				}
			}
		}
	}
}

// classify determines what type of message a webhook event is.
func (m *Messenger) classify(info MessageInfo, e Entry) Action {
	if info.Message != nil {
		return TextAction
	} else if info.Delivery != nil {
		return DeliveryAction
	}

	return UnknownAction
}
