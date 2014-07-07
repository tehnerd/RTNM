package rtnm_pubsub

import (
	"net"
)

type ProbeInfo struct {
	IP       net.IP
	Location string
	Action   string
}
type PubSubMeta struct {
	SubscriberID string //net.IP.String() of subscriber
	Chan         chan ProbeInfo
}

/* pubsub broker which will sends all the data which he has received thru pub_chan
to the subscribers, whos address he has received thru sub chan. optionaly can remove subscriber, whos
address he has received thru unsub_cha */
func StartBroker(sub_chan chan PubSubMeta, unsub_chan chan PubSubMeta,
	pub_chan chan ProbeInfo) {
	pubsub_dict := make(map[string]PubSubMeta)
	for {
		select {
		case new_subscriber := <-sub_chan:
			pubsub_dict[new_subscriber.SubscriberID] = new_subscriber
		case removed_subscriber := <-unsub_chan:
			delete(pubsub_dict, removed_subscriber.SubscriberID)
		case msg := <-pub_chan:
			for ID, Subscriber := range pubsub_dict {
				if ID != msg.IP.String() {
					//will check in action. mb it will be better to make it buffered. so we wont block ourself
					Subscriber.Chan <- msg
				}
			}
		}
	}
}
