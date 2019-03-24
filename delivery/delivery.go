package delivery

import (
	"github.com/hooligram/hooligram-server/actions"
	"github.com/hooligram/hooligram-server/clients"
	"github.com/hooligram/hooligram-server/db"
	"github.com/hooligram/hooligram-server/utils"
)

// MessageDelivery .
type MessageDelivery struct {
	Message      *db.Message
	RecipientIDs []int
}

var messageDeliveryChan = make(chan *MessageDelivery)

// DeliverMessage .
func DeliverMessage() {
	for {
		messageDelivery := <-GetMessageDeliveryChan()
		message := messageDelivery.Message
		recipientIDs := messageDelivery.RecipientIDs

		for _, client := range clients.GetSignedInClients() {
			if !utils.ContainsID(recipientIDs, client.GetID()) {
				continue
			}

			action := actions.CreateMessagingDeliverRequest(message)
			client.WriteJSON(action)
		}
	}
}

// GetMessageDeliveryChan .
func GetMessageDeliveryChan() chan *MessageDelivery {
	return messageDeliveryChan
}
