package main

import (
	"fmt"
	"time"

	"github.com/kataras/iris"
	"github.com/paked/configure"
	"github.com/plimble/messenger"
)

var (
	conf        = configure.New()
	verifyToken = conf.String("verify-token", "somsri-plimble-r422", "The token used to verify facebook")
	pageToken   = conf.String("page-token", "", "The token that is used to verify the page on facebook")
)

func main() {
	server := iris.New()
	conf.Use(configure.NewFlag())
	conf.Use(configure.NewEnvironment())

	conf.Parse()

	// Create a new messenger client
	client := messenger.New(*verifyToken)

	client.OnGetPageToken(func(pageID int64) (string, error) {
		return *pageToken, nil
	})

	client.OnMessage(func(m *messenger.Messenger, msg messenger.Message, r *messenger.Response) {
		fmt.Printf("%v (Sent, %v)\n", msg.Text, msg.Time.Format(time.UnixDate))

		p, err := m.ProfileByID(msg.PageToken, msg.Sender.ID)
		if err != nil {
			fmt.Println("Something went wrong!", err)
		}

		r.Text(fmt.Sprintf("Hello, %v!", p.FirstName))
	})

	// Setup a handler to be triggered when a message is read
	client.OnDelivery(func(m *messenger.Messenger, d messenger.Delivery, r *messenger.Response) {
		fmt.Println(d.Watermark().Format(time.UnixDate))
	})

	server.Get("/webhook", func(c *iris.Context) {
		client.VerifyWebhook(c.ResponseWriter, c.Request)
	})

	server.Post("/webhook", func(c *iris.Context) {
		client.Webhook(c.ResponseWriter, c.Request)
	})

	fmt.Println("Serving messenger bot on localhost:3000")

	server.Listen(":3000")
}
