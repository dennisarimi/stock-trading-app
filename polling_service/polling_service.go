package main

import (
	"bytes"
	"cache"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"

	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type LimitOrder struct {
	Stock  string
	Price  float64
	Type   string
	Amount float64
	User   string `json:"ID"`
	Qty    float64
}

type req struct {
	Sym      string `json:"Sym"`
	Username string `json:"Username"`
}

type quote_hit struct {
	Timestamp int     `json:"Timestamp"`
	Price     float64 `json:"Price"`
	Cryptokey string  `json:"Cryptokey"`
}

var reqUrlPrefix = "http://host.docker.internal:8080"

var active_orders []LimitOrder

func main() {

	// example
	//cache.SetKeyWithExpirationInSecs("foo", 99.8, 0)

	router := gin.Default() // initializing Gin router
	router.SetTrustedProxies(nil)

	router.POST("/new_limit", new_limit)
	router.POST("/quote", get_price)
	bind := flag.String("bind", "localhost:8081", "host:port to listen on")
	flag.Parse()

	if err := router.Run(*bind); err != nil {
		panic(err)
	}

}
func getQuoteUvic(sym string, username string) (float64, int, string) {
	// GET QUOTE FROM UVIC QUOTE SERVER
	strEcho := sym + " " + username + "\n"
	servAddr := "quoteserve.seng.uvic.ca:4444"

	tcpAddr, err := net.ResolveTCPAddr("tcp", servAddr)
	if err != nil {
		fmt.Println("\nResolveTCPAddr error: ", err)
		panic(err)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		fmt.Println("\nDialTCP error: ", err)
		panic(err)
	}

	//write to server SYM being requested and user
	_, err = conn.Write([]byte(strEcho))
	if err != nil {
		fmt.Println("\nWrite error: ", err)
		panic(err)
	}

	//reading from server
	_reply := make([]byte, 1024)

	_, err = conn.Read(_reply)
	if err != nil {
		fmt.Println("\nRead error: ", err)
		panic(err)
	}

	//parsing reply from server
	reply := strings.Split(strings.ReplaceAll(string(_reply), "\n", ""), ",")
	quotePrice, err := strconv.ParseFloat(reply[0], 64)
	if err != nil {
		panic(err)
	}
	timestamp, err := strconv.Atoi(reply[3])
	if err != nil {
		log.Fatal(err)
	}
	cryptKey := reply[4]

	conn.Close()

	return quotePrice, timestamp, cryptKey
}

func quote_price(s string, u string) quote_hit {

	var r req
	r.Username = u
	r.Sym = s
	parsedJson, _ := json.Marshal(r)
	req, err := http.NewRequest(http.MethodPost, "http://quote_server:8083/", bytes.NewBuffer(parsedJson))
	res, err := http.DefaultClient.Do(req)
	reads, err := ioutil.ReadAll(res.Body)
	fmt.Printf("%s \n", reads)
	if err != nil {
		fmt.Println("ERROR")
		fmt.Println(err)
	}

	var m quote_hit
	json.Unmarshal(reads, &m)

	return m
}

func get_price(c *gin.Context) {

	var quote_req req
	if err := c.BindJSON(&quote_req); err != nil {
		c.IndentedJSON(http.StatusOK, err)
		return
	}
	q := quote_price(quote_req.Sym, quote_req.Username)
	fmt.Println("BEFORE CACHE: ")
	fmt.Printf("SYM: %s, USER: %s\n", quote_req.Sym, quote_req.Username)
	fmt.Printf("KEY: %s, VAL: %f\n", quote_req.Sym, q.Price)
	cache.SetKeyWithExpirationInSecs(quote_req.Sym, q.Price, 0)
	c.IndentedJSON(http.StatusOK, q)
}

func do_limit_order() {
	j := 0
	for len(active_orders) > 0 {
		// do: update cache
		val := quote_price(active_orders[j].Stock, active_orders[j].User)

		if val.Price > active_orders[j].Price && active_orders[j].Type == "sell" {
			cache.SetKeyWithExpirationInSecs(active_orders[j].Stock, val.Price, 0)
			//"ID":active_orders[j].User, "Stock": active_orders[j].Stock, "Amount": active_orders[j].Amount, "Price": val
			active_orders[j].Qty = active_orders[j].Amount
			parsedJson, _ := json.Marshal(active_orders[j])
			req, err := http.NewRequest(http.MethodPost, reqUrlPrefix+"/users/sell", bytes.NewBuffer(parsedJson))
			_, err = http.DefaultClient.Do(req)
			req, err = http.NewRequest(http.MethodPost, reqUrlPrefix+"/users/sell/commit", bytes.NewBuffer(parsedJson))
			_, err = http.DefaultClient.Do(req)
			//fmt.Println(res)
			if err != nil {
				fmt.Println("ERROR")
				fmt.Println(err)
			}
			active_orders = append(active_orders[:j], active_orders[j+1:]...)

		} else if val.Price < active_orders[j].Price && active_orders[j].Type == "buy" {
			//writeQuoteToCache(active_orders[j].Stock, active_orders[j].Price)
			cache.SetKeyWithExpirationInSecs(active_orders[j].Stock, val.Price, 0)
			active_orders[j].Qty = active_orders[j].Amount

			parsedJson, _ := json.Marshal(active_orders[j])
			req, err := http.NewRequest(http.MethodPost, reqUrlPrefix+"/users/buy", bytes.NewBuffer(parsedJson))
			res, err := http.DefaultClient.Do(req)

			resBody, err := ioutil.ReadAll(res.Body)
			fmt.Printf("RESBODY: %s\n", resBody)
			if err != nil {
				fmt.Println("ERROR")
				fmt.Println(err)
			}

			req, err = http.NewRequest(http.MethodPost, reqUrlPrefix+"/users/buy/commit", bytes.NewBuffer(parsedJson))
			res, err = http.DefaultClient.Do(req)
			resBody, err = ioutil.ReadAll(res.Body)
			if err != nil {
				fmt.Println("ERROR")
				fmt.Println(err)
			}

			active_orders = append(active_orders[:j], active_orders[j+1:]...)
		}

		time.Sleep(1 * time.Second) // Math goes here
		if len(active_orders) < 1 {
			return
		}
		j = (j + 1) % len(active_orders)
	}

	return
}

func new_limit(c *gin.Context) {
	var limitorder LimitOrder
	if err := c.BindJSON(&limitorder); err != nil {
		c.IndentedJSON(http.StatusOK, err)
		return
	}
	c.IndentedJSON(http.StatusOK, "ok")
	active_orders = append(active_orders, limitorder)
	if len(active_orders) == 1 {
		go do_limit_order()
	} else {
		return
	}

}
