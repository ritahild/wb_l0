package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
	csh  *Cache
	name string
}

func NewDB() *DB {
	db := DB{}
	db.Init()
	return &db
}

// Для обратных вызовов в кеш, сохраняем инстанс *Cache
func (db *DB) SetCahceInstance(csh *Cache) {
	db.csh = csh
}

// Загрузка объектов Orders (кеша) при его восстановлении
func (db *DB) GetCacheState(bufSize int) (map[int64]Order, []int64, int, error) {
	buffer := make(map[int64]Order, bufSize)
	queue := make([]int64, bufSize)
	var queueInd int

	// Выбираем все OrderID для нашей программы (APP_KEY) из таблицы кеша
	query := fmt.Sprintf("SELECT order_id FROM cache WHERE app_key = '%s' ORDER BY id DESC LIMIT %d", os.Getenv("APP_KEY"), bufSize)
	rows, err := db.pool.Query(context.Background(), query)
	if err != nil {
		log.Printf("%v: unable to get order_id from database: %v\n", db.name, err)
	}
	defer rows.Close()

	// Цикл по списку OrderID
	var oid int64
	for rows.Next() {
		if err := rows.Scan(&oid); err != nil {
			log.Printf("%v: unable to get oid from database row: %v\n", db.name, err)
			return buffer, queue, queueInd, errors.New("unable to get oid from database row")
		}
		// сохраняем в очередь в порядке добавления в кеш (перед тем, как программа некоректно завершилась)
		queue[queueInd] = oid
		queueInd++

		o, err := db.GetOrderByID(oid)
		if err != nil {
			log.Printf("%v: unable to get order from database: %v\n", db.name, err)
			continue
		}
		buffer[oid] = o
	}

	if queueInd == 0 {
		return buffer, queue, queueInd, errors.New("cache is empty")
	}

	// переиндексация - в начале queue - "старый" кеш, в конце очереди - "новый". После запроса (самого первого в этой функции) - наоборот
	// Пример: после выполнения кода выше очередь содержит список Order ID: queue = [109 108 107 106 105 104 0 0 0 0],
	// Поскольку id=109 - это более "свежие" данные, то правильный порядок в очереди должен быть такой:
	// queue = [104 105 106 107 108 109 0 0 0 0]
	for i := 0; i < int(queueInd/2); i++ {
		queue[i], queue[queueInd-i-1] = queue[queueInd-i-1], queue[i]
	}

	return buffer, queue, queueInd, nil
}

// Получение Order из БД по id
func (db *DB) GetOrderByID(oid int64) (Order, error) {
	var o Order
	var payment_id_fk int64

	// Сбор данных об Order
	err := db.pool.QueryRow(context.Background(), `SELECT OrderUID, Entry, InternalSignature, Locale, CustomerID, 
	 DeliveryService,payment_id_fk, Shardkey, SmID FROM orders WHERE id = $1`, oid).Scan(
		&o.OrderUID,
		&o.Entry,
		&o.InternalSignature,
		&o.Locale,
		&o.CustomerID,
		&o.DeliveryService,
		&payment_id_fk,
		&o.Shardkey,
		&o.SmID,
	)
	if err != nil {
		return o, errors.New("unable to get order from database")
	}

	// Сбор данных о Payment
	err = db.pool.QueryRow(context.Background(), `SELECT Transaction, Currency, Provider, Amount, PaymentDt, Bank, DeliveryCost,
		GoodsTotal FROM payment WHERE id = $1`, payment_id_fk).Scan(
		&o.Payment.Transaction,
		&o.Payment.Currency,
		&o.Payment.Provider,
		&o.Payment.Amount,
		&o.Payment.PaymentDt,
		&o.Payment.Bank,
		&o.Payment.DeliveryCost,
		&o.Payment.GoodsTotal,
	)
	if err != nil {
		log.Printf("%v: unable to get payment from database: %v\n", db.name, err)
		return o, errors.New("unable to get payment from database")
	}

	// Сбор всех ItemsID для Order
	rowsItems, err := db.pool.Query(context.Background(), "SELECT item_id_fk FROM order_items WHERE order_id_fk = $1", oid)
	if err != nil {
		return o, errors.New("unable to get items id list from database")
	}
	defer rowsItems.Close()

	// Цикл по списку ItemsID
	var itemID int64
	for rowsItems.Next() {
		var item OrderItems
		if err := rowsItems.Scan(&itemID); err != nil {
			return o, errors.New("unable to get itemID from database row")
		}
		// Сбор данных об Items
		err = db.pool.QueryRow(context.Background(), `SELECT ChrtID, Price, Rid, Name, Sale, Size, TotalPrice, NmID, Brand
			FROM items WHERE id = $1`, itemID).Scan(&item.ChrtID, &item.Price, &item.Rid, &item.Name, &item.Sale, &item.Size,
			&item.TotalPrice, &item.NmID, &item.Brand)
		if err != nil {
			return o, errors.New("unable to get item from database")
		}
		o.Items = append(o.Items, item)
	}

	return o, nil
}

// Сохранение Order в БД
func (db *DB) AddOrder(o Order) (int64, error) {
	var lastInsertId int64
	var itemsIds []int64 = []int64{}
	//var orderIdFk int64
	tx, err := db.pool.Begin(context.Background())
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())

	// добавление Items
	for _, item := range o.Items {
		err := tx.QueryRow(context.Background(), `INSERT INTO items (ChrtID, Price, Rid, Name, Sale, Size, TotalPrice, NmID, Brand)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`, item.ChrtID, item.Price, item.Rid, item.Name, item.Sale, item.Size,
			item.TotalPrice, item.NmID, item.Brand).Scan(&lastInsertId)
		if err != nil {
			log.Printf("%v: unable to insert data (items): %v\n", db.name, err)
			return -1, err
		}
		itemsIds = append(itemsIds, lastInsertId)
	}

	// Добавление Payment
	err = tx.QueryRow(
		context.Background(),
		`INSERT INTO payment (Transaction, Currency, Provider, Amount, PaymentDt, Bank, DeliveryCost,
			 GoodsTotal) values ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		o.Payment.Transaction,
		o.Payment.Currency,
		o.Payment.Provider,
		o.Payment.Amount,
		o.Payment.PaymentDt,
		o.Payment.Bank,
		o.Payment.DeliveryCost,
		o.Payment.GoodsTotal,
	).Scan(&lastInsertId)
	if err != nil {
		log.Printf("%v: unable to insert data (payment): %v\n", db.name, err)
		return -1, err
	}

	paymentIdfk := lastInsertId

	// Добавление Order
	err = tx.QueryRow(
		context.Background(),
		`INSERT INTO orders (OrderUID, Entry, InternalSignature,  Locale,
			CustomerID,  DeliveryService,payment_id_fk, Shardkey, SmID) values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id`,
		o.OrderUID,
		o.Entry,
		o.InternalSignature,
		o.Locale,
		o.CustomerID,
		o.DeliveryService,
		paymentIdfk,
		o.Shardkey,
		o.SmID,
	).Scan(&lastInsertId)
	if err != nil {
		log.Printf("%v: unable to insert data (orders): %v\n", db.name, err)
		return -1, err
	}

	orderIdFk := lastInsertId

	// Разрешение связей один-ко-многим для Order и Order.Items[]
	for _, itemId := range itemsIds {
		_, err := tx.Exec(context.Background(), `INSERT INTO order_items (order_id_fk, item_id_fk) values ($1, $2)`,
			orderIdFk, itemId)
		if err != nil {
			log.Printf("%v: unable to insert data (order_items): %v\n", db.name, err)
			return -1, err
		}
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return 0, err
	}

	log.Printf("%v: Order successfull added to DB\n", db.name)
	// После успешной записи добавляем в кеш
	db.csh.SetOrder(orderIdFk, o)
	return orderIdFk, nil
}

// Сохранение в таблицу Cache нового OrderID - нужно для восстановления кеша после сбоя (перед этим сохранили Order в БД и в кеш,
// сохраняем теперь order_id в БД - таблица cahce)
func (db *DB) SendOrderIDToCache(oid int64) {
	db.pool.QueryRow(context.Background(), `INSERT INTO cache (order_id, app_key) VALUES ($1, $2)`, oid, os.Getenv("APP_KEY"))
	log.Printf("%v: OrderID successfull added to Cache (DB)\n", db.name)
}

// Очистка кеша из БД (таблица cache) при корректном завершении программы
func (db *DB) ClearCache() {
	_, err := db.pool.Exec(context.Background(), `DELETE FROM cache WHERE app_key = $1`, os.Getenv("APP_KEY"))
	if err != nil {
		log.Printf("%v: clear cache error: %s\n", db.name, err)
	}
	log.Printf("%v: cache successfull cleared from database\n", db.name)
}
