package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	lnsocket "github.com/jb55/lnsocket/go"
	"github.com/kelseyhightower/envconfig"
	"github.com/tidwall/gjson"
)

type Settings struct {
	CLNHost   string `envconfig:"CLN_HOST"`
	CLNNodeId string `envconfig:"CLN_NODEID"`
	CLNRune   string `envconfig:"CLN_RUNE"`
}

var s Settings

func main() {
	err := envconfig.Process("", &s)
	if err != nil {
		log.Fatal("couldn't process envconfig.")
	}
	http.HandleFunc("/", home)
	http.HandleFunc("/pay", pay)
}

func pay(w http.ResponseWriter, r *http.Request) {
	inv := r.FormValue("invoice")
	cln := lnsocket.LNSocket{}
	cln.GenKey()

	err := cln.ConnectAndInit(s.CLNHost, s.CLNNodeId)
	if err != nil {
		http.Error(w, "failed to connect to node", 500)
		return
	}
	defer cln.Disconnect()

	jparams, _ := json.Marshal(map[string]any{"bolt11": inv})
	result, err := cln.Rpc(s.CLNRune, "pay", string(jparams))
	if err != nil {
		http.Error(w, "failed to call 'pay': "+err.Error(), 600)
		return
	}

	resErr := gjson.Get(result, "error")
	if resErr.Type != gjson.Null {
		msg := fmt.Sprintf("unknown: '%v'", resErr)

		if resErr.Type == gjson.JSON {
			msg = resErr.Get("message").String()
		} else if resErr.Type == gjson.String {
			msg = resErr.String()
		}

		http.Error(w, "failed to pay: "+msg, 400)

		return
	}

	w.Header().Add("content-type", "text/plain")
	fmt.Fprintln(w, "paid!")
	fmt.Fprintln(w, result)
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
<meta charset=utf-8>
<title>invoicepayer</title>
<h1>invoicepayer</h1>
<p>this thing pays lightning invoices on signet, do not abuse</p>
<form>
  <label>
    bolt11 invoice
    <textarea name=invoice />
  </label>
  <button>pay invoice</button>
</form>
<style>
body {
  margin: 10px auto;
  width: 800px;
  max-width: 90%;
}
textarea {
  width: 100%;
  height: 300px;
}
button {
  display: block;
  padding: 2px;
  font-size: 1.5em;
}
</style>
`))
}
