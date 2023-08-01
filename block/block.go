package block

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goblockchain/blockchain_crypto"
	"goblockchain/p2p"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	MINING_DIFFICULITY    = 3
	MINING_SENDER_ADDRESS = "THE BLOCKCHAIN"
	MINING_REWARD         = 1.0
	MINING_TIMER_SEC      = 20

	BLOCKCHAIN_IP_START     = 0
	BLOCKCHAIN_IP_END       = 0
	BLOCKCHAIN_PORT_START   = 5000
	BLOCKCHAIN_PORT_END     = 5003
	NEIGHBOR_SYNC_TIMER_SEC = 20
)

type Block struct {
	nonce        int
	prevHash     [32]byte
	timestamp    int64
	transactions []*Transaction
}

func NewBlock(nonce int, prevHash [32]byte) *Block {
	b := new(Block)
	b.nonce = nonce
	b.prevHash = prevHash
	b.timestamp = time.Now().UnixNano()
	return b
}

func (b *Block) Nonce() int {
	return b.nonce
}

func (b *Block) PrevHash() [32]byte {
	return b.prevHash
}

func (b *Block) Transactions() []*Transaction {
	return b.transactions
}

func (b *Block) Print() {
	fmt.Printf("nonce            %d\n", b.nonce)
	fmt.Printf("prevHash         %x\n", b.prevHash)
	fmt.Printf("timestamp        %d\n", b.nonce)
	for _, t := range b.transactions {
		t.Print()
	}
}

func (b *Block) Hash() [32]byte {
	m, err := json.Marshal(b)
	if err != nil {
		log.Fatal(err)
	}

	sum := sha256.Sum256(m)
	return sum
}

func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Nonce        int            `json:"nonce"`
		PrevHash     string         `json:"prev_hash"`
		Timestamp    int64          `json:"timestamp"`
		Transactions []*Transaction `json:"transactions"`
	}{
		Nonce:        b.nonce,
		PrevHash:     fmt.Sprintf("%x", b.prevHash),
		Timestamp:    b.timestamp,
		Transactions: b.transactions,
	})
}

func (b *Block) UnmarshalJSON(data []byte) error {
	var prevHash string
	v := &struct {
		Nonce        *int            `json:"nonce"`
		PrevHash     *string         `json:"prev_hash"`
		Timestamp    *int64          `json:"timestamp"`
		Transactions *[]*Transaction `json:"transactions"`
	}{
		Nonce:        &b.nonce,
		PrevHash:     &prevHash,
		Timestamp:    &b.timestamp,
		Transactions: &b.transactions,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	ph, _ := hex.DecodeString(*v.PrevHash)
	copy(b.prevHash[:], ph[:32])

	return nil
}

type Blockchain struct {
	transactionPool   []*Transaction
	chain             []*Block
	blockchainAddress string
	port              uint16
	mux               sync.Mutex

	neighbors    []string
	muxNeighbors sync.Mutex
}

func NewBlockchain(blockchainAddress string, port uint16) *Blockchain {
	b := new(Block)
	bc := new(Blockchain)
	bc.CreateBlock(0, b.Hash())
	bc.blockchainAddress = blockchainAddress
	bc.port = port
	return bc
}

func (bc *Blockchain) Chain() []*Block {
	return bc.chain
}

func (bc *Blockchain) TransactionPool() *Transactions {
	return NewTransactions(bc.transactionPool)
}

func (bc *Blockchain) ClearTransactionPool() {
	bc.transactionPool = []*Transaction{}
}

func (bc *Blockchain) Run() {
	bc.StartMining()
	bc.ResolveConflicts()
	bc.SyncNeighbors()
}

func (bc *Blockchain) SetNeighbors() {
	address := p2p.GetHost()
	bc.neighbors = p2p.FindNeighbors(address, bc.port, BLOCKCHAIN_IP_START, BLOCKCHAIN_IP_END, BLOCKCHAIN_PORT_START, BLOCKCHAIN_PORT_END)
	log.Printf("%v", bc.neighbors)
}

func (bc *Blockchain) SyncNeighbors() {
	bc.muxNeighbors.Lock()
	defer bc.muxNeighbors.Unlock()
	bc.SetNeighbors()
	time.AfterFunc(time.Second*NEIGHBOR_SYNC_TIMER_SEC, bc.SyncNeighbors)
}

func (bc *Blockchain) CreateBlock(nonce int, prevHash [32]byte) {
	b := NewBlock(nonce, prevHash)
	b.transactions = bc.transactionPool
	bc.chain = append(bc.chain, b)
	bc.ClearTransactionPool()

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/transactions", n)
		req, _ := http.NewRequest("DELETE", endpoint, nil)
		client := http.Client{}
		resp, _ := client.Do(req)
		log.Printf("%v", resp)
	}
}

func (bc *Blockchain) Print() {
	boundary := strings.Repeat("=", 25)

	for i, b := range bc.chain {
		fmt.Printf("%s Chain %d %s\n", boundary, i, boundary)
		b.Print()
	}
	fmt.Println(strings.Repeat("*", 25))
}

func (bc *Blockchain) LastBlock() *Block {
	return bc.chain[len(bc.chain)-1]
}

func (bc *Blockchain) CreateTransaction(sender, recipient string, value float32, senderPublicKey *ecdsa.PublicKey, signature *blockchain_crypto.Signature) bool {
	isAdded := bc.AddTransaction(sender, recipient, value, senderPublicKey, signature)

	if isAdded {
		publicKeyStr := fmt.Sprintf("%064x%064x", senderPublicKey.X.Bytes(), senderPublicKey.Y.Bytes())
		signatureStr := signature.String()

		for _, n := range bc.neighbors {

			tr := TransactionRequest{
				SenderBlockchainAddress:    &sender,
				RecipientBlockchainAddress: &recipient,
				Value:                      &value,
				PublicKey:                  &publicKeyStr,
				Signature:                  &signatureStr,
			}

			m, _ := json.Marshal(tr)
			buf := bytes.NewBuffer(m)
			endpoint := fmt.Sprintf("http://%s/transactions", n)
			req, _ := http.NewRequest("PUT", endpoint, buf)
			client := http.Client{}
			resp, _ := client.Do(req)

			log.Printf("%v", resp)
		}
	}

	return isAdded
}
func (bc *Blockchain) AddTransaction(sender, recipient string, value float32, senderPublicKey *ecdsa.PublicKey, signature *blockchain_crypto.Signature) bool {
	t := NewTransaction(sender, recipient, value)

	if sender == MINING_SENDER_ADDRESS {
		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}

	if bc.VerifySignature(senderPublicKey, signature, t) {
		if bc.CalculateTotalAmount(sender) < value {
			log.Println("Error: Not enough balance in a wallet")
			return false
		}

		bc.transactionPool = append(bc.transactionPool, t)
		return true
	}

	log.Println("Error: Invalid signature")
	return false
}

func (bc *Blockchain) VerifySignature(senderPublicKey *ecdsa.PublicKey, s *blockchain_crypto.Signature, t *Transaction) bool {
	m, _ := json.Marshal(t)
	h := sha256.Sum256(m)
	return ecdsa.Verify(senderPublicKey, h[:], s.R, s.S)
}

func (bc *Blockchain) CopyTransactions() []*Transaction {
	transactions := make([]*Transaction, 0)

	for _, t := range bc.transactionPool {
		transactions = append(transactions, NewTransaction(
			t.senderBlockchainAddress,
			t.recipientBlockchainAddress,
			t.value,
		))
	}

	return transactions
}

func (bc *Blockchain) ValidProof(nonce int, prevHash [32]byte, transactions []*Transaction, difficulity int) bool {
	guessBlock := Block{
		nonce:        nonce,
		prevHash:     prevHash,
		transactions: transactions,
		timestamp:    0,
	}

	guessHashStr := fmt.Sprintf("%x", guessBlock.Hash())

	return guessHashStr[:difficulity] == strings.Repeat("0", difficulity)
}

func (bc *Blockchain) ProofOfWork() (nonce int) {
	prevHash := bc.LastBlock().Hash()
	transactions := bc.CopyTransactions()

	for !bc.ValidProof(nonce, prevHash, transactions, MINING_DIFFICULITY) {
		nonce++
	}

	return
}

func (bc *Blockchain) Mining() bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// if len(bc.transactionPool) == 0 {
	// 	return false
	// }

	bc.AddTransaction(MINING_SENDER_ADDRESS, bc.blockchainAddress, MINING_REWARD, nil, nil)
	nonce := bc.ProofOfWork()
	prevHash := bc.LastBlock().Hash()
	bc.CreateBlock(nonce, prevHash)
	log.Println("action=Mining, status=success")

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/consensus", n)
		req, _ := http.NewRequest("PUT", endpoint, nil)
		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err)
			return false
		}

		log.Printf("%v\n", resp)
	}

	return true
}

func (bc *Blockchain) StartMining() {
	bc.Mining()
	time.AfterFunc(time.Second*MINING_TIMER_SEC, bc.StartMining)
}

func (bc *Blockchain) CalculateTotalAmount(blockchainAddress string) float32 {
	var total float32 = 0.0

	for _, b := range bc.chain {
		for _, t := range b.transactions {
			value := t.value
			if t.recipientBlockchainAddress == blockchainAddress {
				total += value
			}
			if t.senderBlockchainAddress == blockchainAddress {
				total -= value
			}
		}
	}

	return total
}

func (bc *Blockchain) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Chain []*Block `json:"chain"`
	}{
		Chain: bc.chain,
	})
}

func (bc *Blockchain) UnmarshalJSON(data []byte) error {
	v := &struct {
		Chain *[]*Block `json:"chain"`
	}{
		Chain: &bc.chain,
	}

	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	return nil
}

func (bc *Blockchain) ValidChain(chain []*Block) bool {
	prevBlock := chain[0]
	for i := 1; i < len(chain); i++ {
		currentBlock := chain[i]

		if currentBlock.prevHash != prevBlock.Hash() {
			return false
		}

		if !bc.ValidProof(currentBlock.Nonce(), currentBlock.PrevHash(), currentBlock.Transactions(), MINING_DIFFICULITY) {
			return false
		}

		prevBlock = currentBlock
	}

	return true
}

func (bc *Blockchain) ResolveConflicts() bool {
	var longestChain []*Block
	maxLength := len(bc.chain)

	for _, n := range bc.neighbors {
		endpoint := fmt.Sprintf("http://%s/chain", n)
		resp, _ := http.Get(endpoint)

		if resp.StatusCode == 200 {
			var bcr *Blockchain
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&bcr); err != nil {
				log.Println(err)
				return false
			}

			chain := bcr.Chain()

			if maxLength < len(chain) && bc.ValidChain(chain) {
				longestChain = chain
				maxLength = len(chain)
			}
		} else {
			log.Println("Error: HTTP request error")
			return false
		}
	}

	if longestChain != nil {
		bc.chain = longestChain
		log.Println("Resolve confilicts replaced")
		return true
	}

	log.Println("Resolve confilicts not replaced")
	return false
}

type Transaction struct {
	senderBlockchainAddress    string
	recipientBlockchainAddress string
	value                      float32
}

func NewTransaction(sender, recipient string, value float32) *Transaction {
	t := new(Transaction)
	t.senderBlockchainAddress = sender
	t.recipientBlockchainAddress = recipient
	t.value = value
	return t
}

func (t *Transaction) Print() {
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("senderBlockchainAddress         %s\n", t.senderBlockchainAddress)
	fmt.Printf("recipientBlockchainAddress      %s\n", t.recipientBlockchainAddress)
	fmt.Printf("value                           %.1f\n", t.value)
}

func (t *Transaction) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Sender    string  `json:"sender_blockchain_address"`
		Recipient string  `json:"recipient_blockchain_address"`
		Value     float32 `json:"value"`
	}{
		Sender:    t.senderBlockchainAddress,
		Recipient: t.recipientBlockchainAddress,
		Value:     t.value,
	})
}

func (t *Transaction) UnmarshalJSON(data []byte) error {
	v := &struct {
		Sender    *string  `json:"sender_blockchain_address"`
		Recipient *string  `json:"recipient_blockchain_address"`
		Value     *float32 `json:"value"`
	}{
		Sender:    &t.senderBlockchainAddress,
		Recipient: &t.recipientBlockchainAddress,
		Value:     &t.value,
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}

	return nil
}

type TransactionRequest struct {
	SenderBlockchainAddress    *string  `json:"sender_blockchain_address"`
	RecipientBlockchainAddress *string  `json:"recipient_blockchain_address"`
	Value                      *float32 `json:"value"`
	PublicKey                  *string  `json:"public_key"`
	Signature                  *string  `json:"signature"`
}

func (tr *TransactionRequest) Validate() bool {
	if tr.SenderBlockchainAddress == nil ||
		tr.RecipientBlockchainAddress == nil ||
		tr.Value == nil ||
		tr.PublicKey == nil ||
		tr.Signature == nil {
		return false
	}
	return true
}

type Transactions struct {
	Transactions []*Transaction `json:"transactions"`
	Length       int            `json:"length"`
}

func NewTransactions(transactions []*Transaction) *Transactions {
	return &Transactions{
		Transactions: transactions,
		Length:       len(transactions),
	}
}

type AmountResponse struct {
	Amount float32 `json:"amount"`
}
