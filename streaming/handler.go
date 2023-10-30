package streaming

import (
	"Sirserve/db"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
)

type StreamingHandler struct {
	conn  *stan.Conn
	sub   *Subscriber
	pub   *Publisher
	name  string
	isErr bool
}

func NewStreamingHandler(db *db.DB) *StreamingHandler {
	sh := StreamingHandler{}
	sh.Init(db)
	return &sh
}

func (sh *StreamingHandler) Init(db *db.DB) {
	sh.name = "StreamingHandler"
	err := sh.Connect()

	if err != nil {
		sh.isErr = true
		log.Printf("%s: StreamingHandler error: %s", sh.name, err)
	} else {
		sh.sub = NewSubscriber(db, sh.conn)
		sh.sub.Subscribe()

		sh.pub = NewPublisher(sh.conn)
		sh.pub.Publish()
	}
}

func (sh *StreamingHandler) Connect() error {
	conn, err := stan.Connect(
		os.Getenv("NATS_CLUSTER_ID"),
		os.Getenv("NATS_CLIENT_ID"),
		stan.NatsURL(os.Getenv("NATS_HOSTS")),
		stan.NatsOptions(
			nats.ReconnectWait(time.Second*4),
			nats.Timeout(time.Second*4),
		),
		stan.Pings(5, 3), // Send PINGs every 5 seconds, and fail after 3 PINGs without any response.
		stan.SetConnectionLostHandler(func(_ stan.Conn, reason error) {
			log.Printf("%s: connection lost, reason: %v", sh.name, reason)
		}),
	)
	if err != nil {
		log.Printf("%s: can't connect: %v.\n", sh.name, err)
		return err
	}
	sh.conn = &conn

	log.Printf("%s: connected!", sh.name)
	return nil
}

func (sh *StreamingHandler) Finish() {
	if !sh.isErr {
		log.Printf("%s: Finish...", sh.name)
		sh.sub.Unsubscribe()
		(*sh.conn).Close()
		log.Printf("%s: Finished!", sh.name)
	}
}
