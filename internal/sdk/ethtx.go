package sdk

import "encoding/json"

type EthTx struct {
	TxId string `json:"txId"`
	From string `json:"from"`
	To   string `json:"to"`

	// 交易状态,0 失败 1 成功
	Status uint64 `json:"status"`

	// 交易失败原因
	Msg string `json:"msg"`
}

func (t EthTx) Marshal() ([]byte, error) {
	return json.Marshal(t)
}

func (t EthTx) Unmarshal(bs []byte) error {
	return json.Unmarshal(bs, &t)
}

func (t EthTx) String() string {
	bs, err := t.Marshal()
	if err != nil {
		return ""
	}

	return string(bs)
}
