package mqtt

import "log"

type Subscription struct {
	Topic   string
	Handler func(topic string, payload []byte)
}

var TopicRegistry = []Subscription{
	{
		Topic:   "ebr/sensor/weighing_scale_01",
		Handler: handleWeighing,
	},
}

func handleWeighing(topic string, payload []byte) {
	log.Printf("ВЕСЫ: топик=%s | данные=%s", topic, string(payload))
}
