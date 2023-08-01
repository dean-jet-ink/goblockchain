package main

import (
	"encoding/json"
	"goblockchain/api"
	"goblockchain/block"
	"goblockchain/blockchain_crypto"
	"goblockchain/wallet"
	"io"
	"log"
	"net/http"
	"strconv"
)

var cache map[string]*block.Blockchain = make(map[string]*block.Blockchain)

type BlockchainServer struct {
	port uint16
}

func NewBlockchainServer(port uint16) *BlockchainServer {
	return &BlockchainServer{port}
}

func (bcs *BlockchainServer) Port() uint16 {
	return bcs.port
}

func (bcs *BlockchainServer) GetBlockChain() *block.Blockchain {
	bc, ok := cache["blockChain"]
	if !ok {
		minerWallet := wallet.NewWallet()
		bc = block.NewBlockchain(minerWallet.BlockchainAddress(), bcs.port)
		cache["blockChain"] = bc
		log.Printf("privateKey   %s", minerWallet.PrivateKeyStr())
		log.Printf("publicKey   %s", minerWallet.PublicKeyStr())
		log.Printf("blockChainAddress   %s", minerWallet.BlockchainAddress())
	}

	return bc
}

func (bcs *BlockchainServer) GetChain(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Add("Content-Type", "application/json")
		bc := bcs.GetBlockChain()
		m, _ := json.Marshal(bc)
		io.WriteString(w, string(m))
	} else {
		log.Println("Error: Invalid HTTP Method")
	}
}

func (bcs *BlockchainServer) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bc := bcs.GetBlockChain()
		transactions := bc.TransactionPool()
		m, err := json.Marshal(transactions)
		if err != nil {
			log.Printf("Error: %v\n", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, string(m))

	case http.MethodPost:
		dec := json.NewDecoder(r.Body)
		var btr block.TransactionRequest

		if err := dec.Decode(&btr); err != nil {
			log.Printf("Error: %v\n", err)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}
		if !btr.Validate() {
			log.Println("Error: Missing field(s)")
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}

		publicKey := blockchain_crypto.PublicKeyStrToPublicKey(*btr.PublicKey)
		signature := blockchain_crypto.SignatureStrToSignature(*btr.Signature)

		bc := bcs.GetBlockChain()
		isCreated := bc.CreateTransaction(*btr.SenderBlockchainAddress, *btr.RecipientBlockchainAddress, *btr.Value, publicKey, signature)

		var m []byte
		if isCreated {
			w.WriteHeader(http.StatusCreated)
			m = api.JsonStatus("success")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			m = api.JsonStatus("fail")
		}
		io.WriteString(w, string(m))

	case http.MethodPut:
		dec := json.NewDecoder(r.Body)
		var btr block.TransactionRequest

		if err := dec.Decode(&btr); err != nil {
			log.Printf("Error: %v\n", err)
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}
		if !btr.Validate() {
			log.Println("Error: Missing field(s)")
			io.WriteString(w, string(api.JsonStatus("failed")))
			return
		}

		publicKey := blockchain_crypto.PublicKeyStrToPublicKey(*btr.PublicKey)
		signature := blockchain_crypto.SignatureStrToSignature(*btr.Signature)

		bc := bcs.GetBlockChain()
		isAdded := bc.AddTransaction(*btr.SenderBlockchainAddress, *btr.RecipientBlockchainAddress, *btr.Value, publicKey, signature)

		var m []byte
		if isAdded {
			w.WriteHeader(http.StatusCreated)
			m = api.JsonStatus("success")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			m = api.JsonStatus("fail")
		}
		io.WriteString(w, string(m))

	case http.MethodDelete:
		bcs.GetBlockChain().ClearTransactionPool()

	default:
		log.Println("Error: Invalid HTTP Method")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (bcs *BlockchainServer) Mine(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bc := bcs.GetBlockChain()
		isMined := bc.Mining()
		var m []byte

		if isMined {
			m = api.JsonStatus("success")
		} else {
			w.WriteHeader(http.StatusBadRequest)
			m = api.JsonStatus("fail")
		}

		io.WriteString(w, string(m))
	}
}

func (bcs *BlockchainServer) StartMine(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		bc := bcs.GetBlockChain()
		bc.StartMining()

		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, string(api.JsonStatus("success")))
	default:
		log.Println("Error: Invalid HTTP Method")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (bcs *BlockchainServer) Amount(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		address := r.URL.Query().Get("blockchain_address")

		bc := bcs.GetBlockChain()
		amountValue := bc.CalculateTotalAmount(address)
		amount := &block.AmountResponse{Amount: amountValue}
		m, _ := json.Marshal(amount)

		w.Header().Add("Content-Type", "application/json")
		io.WriteString(w, string(m))
	default:
		log.Println("Error: Invalid HTTP Method")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (bcs *BlockchainServer) Consensus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		bc := bcs.GetBlockChain()
		isResolved := bc.ResolveConflicts()

		if isResolved {
			io.WriteString(w, string(api.JsonStatus("success")))
		} else {
			io.WriteString(w, string(api.JsonStatus("fail")))
		}
	default:
		log.Println("Error: Invalid HTTP Method")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (bcs *BlockchainServer) Start() {
	bcs.GetBlockChain().Run()
	http.HandleFunc("/chain", bcs.GetChain)
	http.HandleFunc("/transactions", bcs.CreateTransaction)
	http.HandleFunc("/mine", bcs.Mine)
	http.HandleFunc("/mine/start", bcs.StartMine)
	http.HandleFunc("/amount", bcs.Amount)
	http.HandleFunc("/consensus", bcs.Consensus)
	http.ListenAndServe(":"+strconv.Itoa(int(bcs.port)), nil)
}
