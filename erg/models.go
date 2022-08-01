package erg

type ErgBoxIds struct {
	Items []ErgTx `json:"items"`
}

type ErgTx struct {
	Id      string        `json:"id"`
	Height  int           `json:"inclusionHeight"`
	Outputs []ErgTxOutput `json:"outputs"`
}

type ErgTxOutput struct {
	BoxId               string    `json:"boxId"`
	AdditionalRegisters Registers `json:"additionalRegisters"`
}

type ErgTxOutputNode struct {
	BoxId               string        `json:"boxId"`
	Assets              []Tokens      `json:"assets"`
	AdditionalRegisters RegistersNode `json:"additionalRegisters"`
	ErgoTree            string        `json:"ergoTree"`
}

type ErgHeader []struct {
	Timestamp int `json:"timestamp"`
	Height    int `json:"height"`
}

type Tokens struct {
	TokenId string `json:"tokenId"`
	Amount  int    `json:"amount"`
}

type Serialized struct {
	BoxId string `json:"boxId"`
	Bytes string `json:"bytes"`
}

type Registers struct {
	R4 Reg `json:"R4"`
	R5 Reg `json:"R5"`
}

type RegistersNode struct {
	R4 string `json:"R4"`
	R5 string `json:"R5"`
	R6 string `json:"R6"`
}

type Reg struct {
	Value string `json:"renderedValue"`
}