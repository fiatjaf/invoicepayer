package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	lightning "github.com/fiatjaf/lightningd-gjson-rpc"
	"github.com/kelseyhightower/envconfig"
	decodepay "github.com/nbd-wtf/ln-decodepay"
	"github.com/tidwall/gjson"
	"github.com/umgefahren/tysyncmap"
	"gopkg.in/antage/eventsource.v1"
)

//go:embed index.html
var indexHtml []byte

//go:embed pay.html
var payHtml []byte

type Settings struct {
	CLN string `envconfig:"CLN"`
}

var (
	s  Settings
	ln *lightning.Client
)

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal("couldn't process envconfig.")
	}

	ln = &lightning.Client{
		Path:        s.CLN,
		CallTimeout: 10 * time.Hour, // optional, defaults to 5 seconds
	}

	http.HandleFunc("/check/", check)
	http.HandleFunc("/pay/", pay)
	http.HandleFunc("/", home)

	fmt.Println("listening on http://127.0.0.1:5556")
	http.ListenAndServe(":5556", nil)
}

var streams = new(tysyncmap.Map[string, eventsource.EventSource])

func check(w http.ResponseWriter, r *http.Request) {
	spl := strings.Split(r.URL.Path, "/")
	hash := spl[len(spl)-1]

	es, ok := streams.Load(hash)

	if !ok {
		closed := false

		es = eventsource.New(
			&eventsource.Settings{
				Timeout:        5 * time.Second,
				CloseOnTimeout: true,
				IdleTimeout:    1 * time.Minute,
			},
			func(r *http.Request) [][]byte {
				return [][]byte{
					[]byte("X-Accel-Buffering: no"),
					[]byte("Cache-Control: no-cache"),
					[]byte("Content-Type: text/event-stream"),
					[]byte("Connection: keep-alive"),
					[]byte("Access-Control-Allow-Origin: *"),
				}
			},
		)
		go func() {
			for {
				time.Sleep(25 * time.Second)
				if closed {
					return
				}
				es.SendEventMessage("", "keepalive", "")
			}
		}()

		go func() {
			time.Sleep(1 * time.Second)
			if closed {
				return
			}
			es.SendRetryMessage(3 * time.Second)
		}()

		go func() {
			time.Sleep(1 * time.Second)
			es.SendEventMessage("connecting", "status", "")
			time.Sleep(1 * time.Second)

			status := "pending"
			es.SendEventMessage(status, "status", "")

			var payment gjson.Result

			for status == "pending" {
				result, err := ln.Call("waitsendpay", map[string]any{"payment_hash": hash})
				if err != nil {
					if closed {
						return
					}
					es.SendEventMessage("error calling 'waitsendpay': "+err.Error(), "status", "")
					return
				}

				status := result.Get("status").String()

				if status == "failed" {
					result, err := ln.Call("listsendpays", map[string]any{"payment_hash": hash})
					if err != nil {
						if closed {
							return
						}
						es.SendEventMessage("error calling 'listsendpays': "+err.Error(), "status", "")
						return
					}

					isPending := false
					for _, p := range result.Get("payments").Array() {
						if p.Get("status").String() == "pending" {
							isPending = true
						} else {
							payment = p
						}
					}

					if isPending {
						status = "pending"
					}
				}

				if closed {
					return
				}
				es.SendEventMessage(status, "status", "")
			}

			if status == "complete" {
				if closed {
					return
				}
				es.SendEventMessage(payment.Raw, "result", "")
			}
		}()

		go func() {
			// check if this subscription has consumers every 2 minutes
			// if not, close it
			for {
				time.Sleep(2 * time.Minute)
				if es.ConsumersCount() == 0 {
					streams.Delete(hash)
					closed = true
					es.Close()
					return // this is important so we exit this loop and don't fall in this condition again
				}
			}
		}()

		streams.Store(hash, es)
	}

	es.ServeHTTP(w, r)
}

func pay(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		w.Write(payHtml)
		return
	}

	inv := r.FormValue("invoice")

	bolt11, err := decodepay.Decodepay(inv)
	if err != nil {
		http.Error(w, "invalid invoice: "+err.Error(), 400)
		return
	}

	returned := false
	go func() {
		result, err := ln.Call("pay", map[string]any{"bolt11": inv})
		if err != nil {
			http.Error(w, "failed to call 'pay': "+err.Error(), 600)
			returned = true
			return
		}

		resErr := result.Get("error")
		if resErr.Type != gjson.Null {
			msg := fmt.Sprintf("unknown: '%v'", resErr)

			if resErr.Type == gjson.JSON {
				msg = resErr.Get("message").String()
			} else if resErr.Type == gjson.String {
				msg = resErr.String()
			}

			http.Error(w, "failed to pay: "+msg, 400)
			returned = true
			return
		}
	}()

	time.Sleep(1 * time.Second)
	if !returned {
		http.Redirect(w, r, "/pay/"+bolt11.PaymentHash, http.StatusFound)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(indexHtml)
}
