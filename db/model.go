package db

type Order struct {
	OrderUID          string       `json:"order_uid"`
	Entry             string       `json:"entry"`
	InternalSignature string       `json:"internal_signature"`
	Locale            string       `json:"locale"`
	CustomerID        string       `json:"customer_id"`
	Items             []OrderItems `json:"items"`

	Delivery      Delivery `json:"delivery"`
	Shardkey      int      `json:"shardkey"`
	SmID          int      `json:"sm_id"`
	TransactionID string   `json:"transaction_id"`
	DataCreated   string   `json:"Data_Created"`
	OffShard      string   `json:"off_shard"`
}

type Payment struct {
	TransactionID string `json:"transaction_id"`
	RequestID     string `json:"request_id"`
	Currency      string `json:"currency"`
	Provider      string `json:"provider"`
	Amount        int    `json:"amount"`
	PaymentDt     int    `json:"payment_dt"`
	Bank          string `json:"bank"`
	DeliveryCost  int    `json:"delivery_cost"`
	GoodsTotal    int    `json:"goods_total"`
}
type Delivery struct {
	DeliveryID int    `json:"delivery_id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Zip        string `json:"zip"`
	City       string `json:"city"`
	Address    string `json:"address"`
	Region     string `json:"region"`
	Email      string `json:"email"`
}

type OrderItems struct {
	OrderUID    string `json:"order_uid"`
	TotalPrice  int    `json:"total_price"`
	TrackNumber string `json:"track_number"`
	NmID        int    `json:"nm_id"`
	Name        string `json:"name"`
	Sale        int    `json:"sale"`
	Size        string `json:"size"`
	Price       int    `json:"price"`
	Rid         string `json:"rid"`
	Brand       string `json:"brand"`
	ChrtID      int    `json:"chrt_id"`
	Status      string `json:"status"`
}
