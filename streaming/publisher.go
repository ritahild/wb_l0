package streaming

import (
	"Sirserve/db"
	"encoding/json"
	"strconv"

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

	item1 := db.OrderItems{
		OrderUID:    strconv.Itoa(1),
		TotalPrice:  13,
		TrackNumber: "2",
		NmID:        1,
		Name:        "T-Shirt-4",
		Sale:        9,
		Size:        "M",
		Price:       10,
		Rid:         "rid 1",
		Brand:       "Adidas",
		ChrtID:      1,
		Status:      "red",
	}
	item2 := db.OrderItems{
		OrderUID:    strconv.Itoa(2),
		TotalPrice:  14,
		TrackNumber: "3",
		NmID:        2,
		Name:        "Jeans",
		Sale:        11,
		Size:        "S",
		Price:       12,
		Rid:         "rid 2",
		Brand:       "Collins",
		ChrtID:      2,
		Status:      "gren",
	}
	payment := db.Payment{
		Transaction: "tran 1",

		Currency:     "Rub",
		Provider:     "Provider 1",
		Amount:       47,
		PaymentDt:    2,
		Bank:         "VTB",
		DeliveryCost: 7,
		GoodsTotal:   3,
	}

	order := db.Order{
		OrderUID:          strconv.Itoa(2),
		Entry:             "2",
		InternalSignature: "IS 2",
		Locale:            "Ru",
		CustomerID:        "2",
		DeliveryService:   "meest",
		Payment:           payment,
		Items:             []db.OrderItems{item1, item2},

		Shardkey: "6",
		SmID:     2,
	}
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
