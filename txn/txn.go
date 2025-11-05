package txn

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

//golobal assest (vehicles for sale etc)
type Asset struct {
	id          int32
	name        string
	filename    string
	Description string
	price       uint32
}

type Possesion struct {
	//playerdId int32
	assetId          *Asset
	Name             string //defaults to/overrides asset.name
	destroyedInGames []uint32
	TxnId            uint32 //purchase price,  date
}
