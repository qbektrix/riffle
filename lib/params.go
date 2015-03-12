package lib

import (
	"github.com/dedis/crypto/abstract"
	//"github.com/dedis/crypto/openssl"
	"github.com/dedis/crypto/edwards"
)

var Suite abstract.Suite = edwards.NewAES128SHA256Ed25519(false)
//var Suite abstract.Suite = openssl.NewAES128SHA256P256()
const NumClients = 5
const NumServers = 2

const MaxRounds = 3

//sizes in bytes
const HashSize = 32
const BlockSize = 1024*1024 //1KB for testing;
//const BlockSize = 1024*1024 //1MB
const SecretSize = 256/8


var ServerAddrs []string = []string{"localhost:8000", "localhost:8001"}
const ServerPort = 8000

const ClientPort = 9000
