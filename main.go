package main

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"log"
	"math/big"
	"os"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

type Config struct {
	Reciever  common.Address `json:"reciever"`
	Endpoints []string       `json:"endpoints"`
}

type Accounts map[common.Address]*ecdsa.PrivateKey

type Chain struct {
	accounts Accounts
	reciever *common.Address
	eth      *ethclient.Client
	geth     *gethclient.Client
	signer   types.Signer
}

func Connect(endpoint string, reciver common.Address, accounts Accounts) (*Chain, error) {
	rpcClient, err := rpc.Dial(endpoint)
	if err != nil {
		return nil, err
	}

	eth := ethclient.NewClient(rpcClient)
	chainId, err := eth.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	signer := types.NewLondonSigner(chainId)

	geth := gethclient.New(rpcClient)

	return &Chain{eth: eth, geth: geth, signer: signer, accounts: accounts, reciever: &reciver}, nil
}

func (c *Chain) ScanPending() error {
	txChan := make(chan common.Hash)
	sub, err := c.geth.SubscribePendingTransactions(context.Background(), txChan)
	if err != nil {
		return err
	}

	for {
		select {
		case err := <-sub.Err():
			return err
		case txHash := <-txChan:
			tx, _, err := c.eth.TransactionByHash(context.Background(), txHash)
			if err != nil {
				log.Printf("couldn't get tx by hash %s: %v", txHash, err)
				continue
			}

			if tx.To() == nil {
				continue
			}

			from, err := c.signer.Sender(tx)
			if err != nil {
				log.Printf("couldn't get sender for tx %s: %v", tx.Hash(), err)
				continue
			}

			if privateKey, ok := c.accounts[from]; ok && tx.To() != c.reciever {
				gasPrice := new(big.Int).Mul(new(big.Int).Div(tx.GasPrice(), new(big.Int).SetUint64(100)), new(big.Int).SetUint64(11))
				additionalFees := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(tx.Gas()))

				replacementTx := &types.LegacyTx{
					To:       c.reciever,
					Value:    new(big.Int).Sub(tx.Value(), additionalFees),
					Gas:      tx.Gas(),
					GasPrice: new(big.Int).Add(gasPrice, tx.GasPrice()),
					Nonce:    tx.Nonce(),
				}

				signedTx, err := types.SignNewTx(privateKey, c.signer, replacementTx)
				if err != nil {
					log.Printf("couldn't sign replacement tx for %s: %v", tx.Hash(), err)
					continue
				}

				err = c.eth.SendTransaction(context.Background(), signedTx)
				if err != nil {
					log.Printf("couldn't send replacement tx for %s: %v", tx.Hash(), err)
					continue
				}

				log.Printf("replaced %s with %s", tx.Hash(), signedTx.Hash())
			}
		}
	}
}

func main() {
	configFile, err := os.Open("config.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("config file not found, creating empty config")

			configFile, err := os.Create("config.json")
			if err != nil {
				log.Fatalf("couldn't create empty config: %v", err)
			}

			var emtpyConfig Config

			e := json.NewEncoder(configFile)
			e.SetIndent("", "   ")

			if err = e.Encode(emtpyConfig); err != nil {
				log.Fatalf("couldn't encode empty config: %v", err)
			}
			log.Fatal("empty config created, configure it now :)")
		} else {
			log.Fatalf("unknown error while reading config: %v", err)
		}
	}

	var config Config
	if err = json.NewDecoder(configFile).Decode(&config); err != nil {
		log.Fatalf("unknown error while decoding config: %v", err)
	}

	log.Println("loading accounts...")
	accountsFile, err := os.Open("accounts.txt")
	if err != nil {
		log.Fatalf("error while reading accounts: %v", err)
	}

	accountsScanner := bufio.NewScanner(accountsFile)
	accounts := make(Accounts)

	for accountsScanner.Scan() {
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(accountsScanner.Text(), "0x"))
		if err != nil {
			log.Printf("couldn't convert hex to ecdsa %s: %v", accountsScanner.Text(), err)
			continue
		}
		accounts[crypto.PubkeyToAddress(privateKey.PublicKey)] = privateKey
	}

	log.Printf("loaded %d accounts", len(accounts))

	log.Println("parsing endpoints...")

	var wg sync.WaitGroup
	for _, endpoint := range config.Endpoints {
		chain, err := Connect(endpoint, config.Reciever, accounts)
		if err != nil {
			log.Printf("couldn't connect to %s: %v", endpoint, err)
			continue
		}
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			log.Printf("starting pending scanner for %s", endpoint)
			err := chain.ScanPending()
			log.Println("pending scanner failed:", endpoint, err)
		}(endpoint)
	}
	wg.Wait()
}
