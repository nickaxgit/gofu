package txn

import "vendor/golang.org/x/net/idna"

type Txn struct {
	id           uint32
	fromPlayerId uint32
	toPlayerId   uint32
	amount       uint32
	reference    string
	balanceAfter uint32
	assetId      int32
	timeStamp    int64
}

type Asset struct {
	id          int32
	name        string
	filename    string
	Description string
	price       uint32
}

type possesion struct {
	//playerdId int32
	assetId          *Asset
	Name             string //defaults to/overrises asset.name
	destroyedInGames []uint32
	TxnId            uint32 //purchase price,  date
}
