package db

import (
	"log"
	"os"
	"strconv"
	"sync"
)

type Cache struct {
	buffer  map[int64]Order
	queue   []int64
	bufSize int
	pos     int
	DBInst  *DB
	name    string
	mutex   *sync.RWMutex
}

func NewCache(db *DB) *Cache {
	csh := Cache{}
	csh.Init(db)
	return &csh
}

// Инициализация кеша - установка размера, восстанавление
func (c *Cache) Init(db *DB) {
	c.DBInst = db
	db.SetCahceInstance(c)
	c.name = "Cahce"
	c.mutex = &sync.RWMutex{}

	// Установка размера кеша
	bufSize, err := strconv.Atoi(os.Getenv("CACHE_SIZE"))
	if err != nil {
		log.Printf("%s: Init() warning: set default cache size 10\n", c.name)
		bufSize = 10
	}

	c.bufSize = bufSize
	c.buffer = make(map[int64]Order, c.bufSize)
	c.queue = make([]int64, c.bufSize)

	// Восстанавление кеша из базы данных, если он есть в бд
	c.getCacheFromDatabase()
}

// Восстанавливаем кеш из базы данных: читаем из файла содержимое кеша
func (c *Cache) getCacheFromDatabase() {
	log.Printf("%v: check & download cache from database\n", c.name)
	buf, queue, pos, err := c.DBInst.GetCacheState(c.bufSize)
	if err != nil {
		log.Printf("%s: getCacheFromDatabase() warning: can't download from database or cache is empty: %v\n", c.name, err)
		return
	}

	// Проверяем, не заполнили ли буфер полностью. Если да - сбрасываем указатель на начало циклической очереди
	if pos == c.bufSize {
		pos = 0
	}

	c.mutex.Lock()
	c.buffer = buf
	c.queue = queue
	c.pos = pos
	c.mutex.Unlock()
	log.Printf("%s: cache downloaded from database: queue is: %v, next position in queue is: %v", c.name, c.queue, c.pos)
}

// Сохранение в кеш после успешного добавления Order в БД
func (c *Cache) SetOrder(oid int64, o Order) {
	if c.bufSize > 0 {
		c.mutex.Lock()
		// сохраняем в циклическую очередь новый orderId (если на позиции pos будет Order, он будет перезаписан)
		c.queue[c.pos] = oid
		c.pos++
		if c.pos == c.bufSize {
			c.pos = 0
		}

		// сохраняем в буфер новый Order
		c.buffer[oid] = o
		c.mutex.Unlock()

		// сохраняем в таблицу Cache в БД новый OrderID - для восстановления кеша после сбоя
		c.DBInst.SendOrderIDToCache(oid)
		log.Printf("%s: Order successfull added to Cahce, Order position in queue is %v\n", c.name, c.pos)
	} else {
		log.Printf("%s: cache is off: bufSize = 0 (see config.go)\n", c.name)
	}
	//fmt.Println(c.buffer)
	log.Printf("%s: queue is: %v, next position in queue is: %v", c.name, c.queue, c.pos)
}

// Получаем Order по ID из кеша. Преобразование к модели для выдачи
func (c *Cache) GetOrderOutById(oid int64) (*OrderItems, error) {
	var ou *OrderItems = &OrderItems{}
	var o Order
	var err error

	c.mutex.RLock()
	// проверка в кеше. Если нет - идем в базу
	o, isExist := c.buffer[oid]
	c.mutex.RUnlock()

	if isExist {
		log.Printf("%s: OrderItems (id:%d) взят из кеша!\n", c.name, oid)
	} else {
		// запрос Order к базе данных
		o, err = c.DBInst.GetOrderByID(oid)
		if err != nil {
			log.Printf("%s: GetOrderOutById(): ошибка получения Order: %v\n", c.name, err)
			return ou, err
		}
		// Сохранение в кеш
		c.SetOrder(oid, o)
		log.Printf("%s: OrderItems (id:%d) взят из бд и сохранен в кеш!\n", c.name, oid)
	}

	// Преобразование к модели для выдачи

	ou.OrderUID = o.OrderUID

	return ou, nil
}

func (c *Cache) Finish() {
	log.Printf("%s: Finish...", c.name)
	c.DBInst.ClearCache()
	log.Printf("%s: Finished", c.name)
}
