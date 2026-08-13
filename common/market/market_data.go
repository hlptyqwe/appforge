package cache

import (
	"strings"
)

func BuildTopicKey(msg ClientMessage) string {
	msg = NormalizeClientMessage(msg)
	return strings.ToLower(string(msg.Topic) + ":" + msg.CategoryCode + ":" + msg.Symbol + ":" + msg.Market + ":" + msg.Interval)
}

func NormalizeClientMessage(msg ClientMessage) ClientMessage {
	msg.Topic = Topic(strings.ToLower(strings.TrimSpace(string(msg.Topic))))
	msg.CategoryCode = strings.ToLower(strings.TrimSpace(msg.CategoryCode))
	msg.Symbol = strings.ToUpper(strings.TrimSpace(msg.Symbol))
	msg.Market = strings.ToUpper(strings.TrimSpace(msg.Market))
	msg.Interval = strings.ToLower(strings.TrimSpace(msg.Interval))
	if msg.Topic != TopicKline {
		msg.Interval = ""
	}
	return msg
}
