package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/hooligram/hooligram-server/actions"
	"github.com/hooligram/hooligram-server/clients"
	"github.com/hooligram/hooligram-server/delivery"
	"github.com/hooligram/hooligram-server/utils"
	"github.com/hooligram/hooligram-server/v2"
)

const mainTag = "main"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal(fmt.Sprintf("[%v] PORT not set", mainTag))
	}

	router := mux.NewRouter()
	router.HandleFunc("/v2", v2.V2)

	go deliverMessage()

	http.ListenAndServe(":"+port, router)
}

func deliverMessage() {
	for {
		messageDelivery := <-delivery.GetMessageDeliveryChan()
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
