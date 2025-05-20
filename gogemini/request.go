package gogemini

type RequestGetAnswer struct {
	At   string `json:"at" required:"true" validate:"nonnil,min=1"`
	FReq string `json:"f.req" required:"true" validate:"nonnil,min=1"`
}
