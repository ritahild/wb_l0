package streaming

import (
	"Sirserve/db"
	"encoding/json"
	"log"
	"os"

	stan "github.com/nats-io/stan.go"
)

type Publisher struct {
	sc   *stan.Conn
	name string
}

func NewPublisher(conn *stan.Conn) *Publisher {
	return &Publisher{
		name: "Publisher",
		sc:   conn,
	}
}

func (p *Publisher) Publish() {

	item1 := db.OrderItems{ChrtID: 1, TrackNumber: "2", Price: 10, Rid: "rid 1", Name: "T-Shirt-4", Sale: 9, Size: "M", TotalPrice: 13, NmID: 1, Brand: "Adidas"}
	item2 := db.OrderItems{ChrtID: 2, Price: 12, Rid: "rid 2", Name: "Jeans", Sale: 11, Size: "S", TotalPrice: 14, NmID: 2, Brand: "Collins"}
	item3 := db.OrderItems{ChrtID: 3, Price: 18, Rid: "rid 3", Name: "Sneakers", Sale: 15, Size: "M", TotalPrice: 20, NmID: 1, Brand: "Nike"}

	order := db.Order{OrderUID: "Order 2", Entry: "2", InternalSignature: "IS 2", Items: []db.OrderItems{item1, item2, item3},
		Locale: "Ru", CustomerID: "2", SmID: 2}
	orderData, err := json.Marshal(order)
	if err != nil {
		log.Printf("%s: json.Marshal error: %v\n", p.name, err)
	}

	ackHandler := func(ackedNuid string, err error) {
		if err != nil {
			log.Printf("%s: error publishing msg id %s: %v\n", p.name, ackedNuid, err.Error())
		} else {
			log.Printf("%s: received ack for msg id: %s\n", p.name, ackedNuid)
		}
	}

	log.Printf("%s: publishing data ...\n", p.name)
	nuid, err := (*p.sc).PublishAsync(os.Getenv("NATS_SUBJECT"), orderData, ackHandler) // returns immediately
	if err != nil {
		log.Printf("%s: error publishing msg %s: %v\n", p.name, nuid, err.Error())
	}
}
