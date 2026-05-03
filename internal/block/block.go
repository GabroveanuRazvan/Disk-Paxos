package block

import "encoding/json"

type Block struct {
	Mbal int    `json:"mbal"` // Current ballot number
	Bal  int    `json:"bal"`  // Highest commited ballot number
	Inp  string `json:"inp"`  // Value to be proposed
}

func (p *Block) JSON() string {
	b, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	return string(b)
}
