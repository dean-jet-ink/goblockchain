package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"goblockchain/api"
	"goblockchain/block"
	"goblockchain/blockchain_crypto"
	"goblockchain/wallet"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
)

var templDir = "templates"

type WalletServer struct {
	port    uint16
	gateway string
}

func NewWalletServer(port uint16, gateway string) *WalletServer {
	return &WalletServer{port, gateway}
}

func (ws *WalletServer) Port() uint16 {
	return ws.port
}

func (ws *WalletServer) Gateway() string {
	return ws.gateway
}

func (ws *WalletServer) Index(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		t, err := template.ParseFiles(path.Join(templDir, "index.html"))
		if err != nil {
			log.Println(err)
			return
		}
		t.Execute(w, "")
	default:
		log.Println("Error: Invalid HTTP Method")
	}
}

func (ws *WalletServer) Wallet(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		w.Header().Add("Content-Type", "application/json")
		myWallet := wallet.NewWallet()
		m, _ := json.Marshal(myWallet)
		io.WriteString(w, string(m))
	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("Error: Invalid HTTP Method")
	}
}

func (ws *WalletServer) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		dec := json.NewDecoder(r.Body)
		var tr wallet.TransactionRequest
		if err := dec.Decode(&tr); err != nil {
			log.Printf("Error: %v\n", err)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}
		if !tr.Validate() {
			log.Println("Error: Missing field(s)")
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}

		publicKey := blockchain_crypto.PublicKeyStrToPublicKey(*tr.SenderPublicKey)
		privateKey := blockchain_crypto.PrivateKeyStrToPrivateKey(*tr.SenderPrivateKey, publicKey)
		value64, err := strconv.ParseFloat(*tr.Value, 32)
		if err != nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}
		value := float32(value64)

		transaction := wallet.NewTransaction(privateKey, publicKey, *tr.SenderBlockchainAddress, *tr.RecipientBlockchainAddress, value)
		signature := transaction.GenerateSignature()
		signatureStr := signature.String()

		btr := block.TransactionRequest{
			SenderBlockchainAddress:    tr.SenderBlockchainAddress,
			RecipientBlockchainAddress: tr.RecipientBlockchainAddress,
			Value:                      &value,
			PublicKey:                  tr.SenderPublicKey,
			Signature:                  &signatureStr,
		}
		m, _ := json.Marshal(btr)
		buf := bytes.NewBuffer(m)

		response, err := http.Post(ws.Gateway()+"/transactions", "application/json", buf)
		if response == nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}

		if response.StatusCode == 201 {
			io.WriteString(w, string(api.JsonStatus("success")))
			return
		} else {
			w.WriteHeader(response.StatusCode)
			log.Printf("Error: %v\n", err)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("Error: Invalid HTTP Method")
	}
}

func (ws *WalletServer) WalletAmount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		blockchainAddress := r.URL.Query().Get("blockchain_address")

		endpoint := fmt.Sprintf("%s/amount", ws.gateway)
		bcsReq, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, string(api.JsonStatus("fail")))
			return
		}
		q := bcsReq.URL.Query()
		q.Add("blockchain_address", blockchainAddress)
		bcsReq.URL.RawQuery = q.Encode()

		client := &http.Client{}
		response, err := client.Do(bcsReq)
		if err != nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, string(api.JsonStatus("fail")))
			return
		}

		w.Header().Add("Content-Type", "application/json")
		if response.StatusCode == 200 {
			dec := json.NewDecoder(response.Body)
			var amount block.AmountResponse
			if err := dec.Decode(&amount); err != nil {
				log.Printf("Error: %v\n", err)
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, string(api.JsonStatus("fail")))
				return
			}

			m, _ := json.Marshal(struct {
				Message string  `json:"message"`
				Amount  float32 `json:"amount"`
			}{
				Message: "success",
				Amount:  amount.Amount,
			})

			io.WriteString(w, string(m))
		} else {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, string(api.JsonStatus("fail")))
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
		log.Println("Error: Invalid HTTP Method")
	}
}

func (ws *WalletServer) Start() {
	http.HandleFunc("/", ws.Index)
	http.HandleFunc("/wallet", ws.Wallet)
	http.HandleFunc("/wallet/amount", ws.WalletAmount)
	http.HandleFunc("/transaction", ws.CreateTransaction)
	http.ListenAndServe(":"+strconv.Itoa(int(ws.port)), nil)
}
