package mqtt

import (
	"log/slog"
	"time"

	mqttlib "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client        mqttlib.Client
	subscriptions []Subscription
}

func NewClient(brokerURL, clientID string, subscriptions []Subscription) *Client {
	opts := mqttlib.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID(clientID)
	opts.OnConnect = func(c mqttlib.Client) {
		for _, sub := range subscriptions {
			// type MessageHandler func(Client, Message)
			// sub.Handler - callback функция, будет выполняться, когда придут данные
			token := c.Subscribe(sub.Topic, 1, func(_ mqttlib.Client, msg mqttlib.Message) {
				sub.Handler(msg.Topic(), msg.Payload())
			})
			if token.WaitTimeout(5*time.Second) && token.Error() != nil {
				slog.Error("failed to subscribe", "topic", sub.Topic, "err", token.Error())
			} else {
				slog.Info("successfully subscribed", "topic", sub.Topic)
			}
		}
	}
	c := mqttlib.NewClient(opts)
	return &Client{
		client:        c,
		subscriptions: subscriptions,
	}
}

func (c *Client) Connect() error {
	if token := c.client.Connect(); token.WaitTimeout(10*time.Second) && token.Error() != nil {
		return token.Error()
	}
	slog.Info("MQTT client connected")
	return nil
}

func (c *Client) Disconnect(timeout uint) {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(timeout)
		slog.Info("MQTT client disconnected")
	}
}
