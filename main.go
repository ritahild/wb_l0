package main

import (
	. "Sirserve/config"
	"Sirserve/db"
	_ "Sirserve/server"
	"Sirserve/streaming"
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"

	"os"
	"os/signal"

	"html/template"
	"log"
	"net/http"
	"strconv"
	"sync"
)

func main() {

	ConfigSetup()
	dbObject := db.NewDB()
	csh := db.NewCache(dbObject)
	sh := streaming.NewStreamingHandler(dbObject)

	myJoint := NewJoint(csh)

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			csh.Finish()
			sh.Finish()
			myJoint.Finish()

			cleanupDone <- true
		}
	}()
	<-cleanupDone
}

type ordkey string

const orderKey ordkey = "order"

type Joint struct {
	rtr                *chi.Mux
	csh                *db.Cache
	name               string
	srv                *http.Server
	httpServerExitDone *sync.WaitGroup
}

func NewJoint(csh *db.Cache) *Joint {
	api := Joint{}
	api.Init(csh)
	return &api
}

func (a *Joint) Init(csh *db.Cache) {
	a.csh = csh
	a.name = "API"
	a.rtr = chi.NewRouter()
	a.rtr.Get("/", a.WellcomeHandler)

	a.rtr.Route("/orders", func(r chi.Router) {
		r.Route("/{orderID}", func(r chi.Router) {
			r.Use(a.orderCtx)
			r.Get("/", a.GetOrder) // GET /orders/123
		})
	})

	a.httpServerExitDone = &sync.WaitGroup{}
	a.httpServerExitDone.Add(1)
	a.StartServer()
}

func (a *Joint) Finish() {
	log.Printf("%v: Выключение сервера...\n", a.name)

	if err := a.srv.Shutdown(context.Background()); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}

	a.httpServerExitDone.Wait()
	log.Printf("%v: Сервер успешно выключен!\n", a.name)
}

func (a *Joint) StartServer() {
	a.srv = &http.Server{
		Addr:    ":3333",
		Handler: a.rtr,
	}

	go func() {
		defer a.httpServerExitDone.Done() // let main know we are done cleaning up

		log.Printf("%v: сервер будет запущен по адресу http://localhost:3333\n", a.name)

		if err := a.srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Printf("ListenAndServe() error: %v", err)
			return
		}
	}()
}

func (a *Joint) orderCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orderIDstr := chi.URLParam(r, "orderID")
		orderID, err := strconv.ParseInt(orderIDstr, 10, 64)
		if err != nil {
			log.Printf("%v: ошибка конвертации %s в число: %v\n", a.name, orderIDstr, err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		log.Printf("%v: запрос OrderItems из кеша/бд, OrderID: %v\n", a.name, orderIDstr)
		orderitems, err := a.csh.GetOrderOutById(orderID)
		if err != nil {
			log.Printf("%v: ошибка получения OrderItems из базы данных: %v\n", a.name, err)
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound) // 404
			return
		}
		ctx := context.WithValue(r.Context(), orderKey, orderitems)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Joint) WellcomeHandler(w http.ResponseWriter, r *http.Request) {

	t, err := template.ParseFiles("ui/templates/orders.html")
	if err != nil {
		log.Printf("%v: getOrder(): ошибка парсинга шаблона html: %s\n", a.name, err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
	err = t.ExecuteTemplate(w, "orders.html", nil)
	if err != nil {
		log.Printf("%v: WellcomeHandler(): ошибка выполнения шаблона html: %s\n", a.name, err)
		return
	}
}

func (a *Joint) GetOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	OrderItems, ok := ctx.Value(orderKey).(*db.OrderItems)
	if !ok {
		log.Printf("%v: getOrder(): ошибка приведения интерфейса к типу *OrderItems\n", a.name)
		http.Error(w, http.StatusText(http.StatusUnprocessableEntity), http.StatusUnprocessableEntity) // 422
		return
	}

	t, err := template.ParseFiles("ui/templates/orders.html")
	if err != nil {
		log.Printf("%v: getOrder(): ошибка парсинга шаблона html: %s\n", a.name, err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
	t.ExecuteTemplate(w, "orders.html", OrderItems)
	if err != nil {
		log.Printf("%v: GetOrder(): ошибка выполнения шаблона html: %s\n", a.name, err)
		return
	}
}
