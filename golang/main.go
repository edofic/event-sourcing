package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"sort"
	"strconv"
)

const MAX_ORDERBOOK_SIZE = 100

type Orderbook struct {
	Asks []Order
	Bids []Order
}

type Order struct {
	User      UserId
	OrderType int
	Price     Price
}

type UserId = int
type Price = int

const (
	OrderTypeBuy  = iota
	OrderTypeSell = iota
)

func (ob *Orderbook) placeOrder(order Order) {
	switch order.OrderType {
	case OrderTypeBuy:
		ob.Bids = append(ob.Bids, order)
		sort.Slice(ob.Bids, func(i, j int) bool {
			return ob.Bids[i].Price < ob.Bids[j].Price
		})
	case OrderTypeSell:
		ob.Asks = append(ob.Asks, order)
		sort.Slice(ob.Asks, func(i, j int) bool {
			return ob.Asks[i].Price > ob.Asks[j].Price
		})
	default:
		panic("Unknown order type")
	}
	if len(ob.Asks) > 0 && len(ob.Bids) > 0 {
		if ob.Bids[len(ob.Bids)-1].Price >= ob.Asks[len(ob.Asks)-1].Price {
			ob.Bids = ob.Bids[:len(ob.Bids)-1]
			ob.Asks = ob.Asks[:len(ob.Asks)-1]
		}
	}
	if len(ob.Asks) > MAX_ORDERBOOK_SIZE {
		ob.Asks = ob.Asks[1:]
	}
	if len(ob.Bids) > MAX_ORDERBOOK_SIZE {
		ob.Bids = ob.Bids[1:]
	}

}

func (ob *Orderbook) Copy() Orderbook {
	asks := make([]Order, len(ob.Asks))
	copy(asks, ob.Asks)
	bids := make([]Order, len(ob.Bids))
	copy(bids, ob.Bids)
	return Orderbook{asks, bids}
}

func runNewManagedState() *ManagedState {
	s := &ManagedState{make(chan Order), make(chan (chan Orderbook), 1)}
	go runEventloop(s.orders, s.snapshot)
	return s
}

func runEventloop(orders chan Order, snapshot chan (chan Orderbook)) {
	orderbook := &Orderbook{make([]Order, 0), make([]Order, 0)}
	for {
		select {
		case order := <-orders:
			orderbook.placeOrder(order)

		case snapshotCh := <-snapshot:
			snapshotCh <- orderbook.Copy()
		}
	}
}

type ManagedState struct {
	orders   chan Order
	snapshot chan (chan (Orderbook))
}

func (s *ManagedState) dispatch(order Order) {
	s.orders <- order
}

func (s *ManagedState) get() Orderbook {
	ch := make(chan (Orderbook))
	s.snapshot <- ch
	return <-ch
}

func main() {
	s := runNewManagedState(initial)
	router := mux.NewRouter()
	router.HandleFunc("/", Index)
	router.HandleFunc("/orderbook", ViewOrderbook(s))
	router.HandleFunc("/buy/{userId:[0-9]+}/{price:[0-9]+}", PlaceBuy(s))
	router.HandleFunc("/sell/{userId:[0-9]+}/{price:[0-9]+}", PlaceSell(s))
	log.Fatal(http.ListenAndServe(":8080", router))
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello")
}

func ViewOrderbook(s *ManagedState) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(s.get())
	}
}

func PlaceBuy(s *ManagedState) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userId, _ := strconv.Atoi(vars["userId"])
		price, _ := strconv.Atoi(vars["price"])
		s.dispatch(Order{userId, OrderTypeBuy, price})
		fmt.Fprintln(w, "ok")
	}
}

func PlaceSell(s *ManagedState) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		userId, _ := strconv.Atoi(vars["userId"])
		price, _ := strconv.Atoi(vars["price"])
		s.dispatch(Order{userId, OrderTypeSell, price})
		fmt.Fprintln(w, "ok")
	}
}
