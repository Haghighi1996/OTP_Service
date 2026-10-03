package internal

type Part struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Weight   float64 `json:"weight"`
	Quantity int     `json:"quantity"`
}
