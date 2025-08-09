package models

type Card struct {
	ID     int  `json:"id"`
	Type   int  `json:"type"`
	IsReal bool `json:"is_real"`
}

var AvailableRealCards = []Card{
	{ID: 1, Type: 1, IsReal: true},
	{ID: 2, Type: 1, IsReal: true},
	{ID: 3, Type: 1, IsReal: true},
	{ID: 4, Type: 1, IsReal: true},
	{ID: 1, Type: 2, IsReal: true},
	{ID: 2, Type: 2, IsReal: true},
	{ID: 3, Type: 2, IsReal: true},
	{ID: 4, Type: 2, IsReal: true},
	{ID: 1, Type: 3, IsReal: true},
	{ID: 2, Type: 3, IsReal: true},
	{ID: 3, Type: 3, IsReal: true},
	{ID: 4, Type: 3, IsReal: true},
	{ID: 1, Type: 4, IsReal: true},
	{ID: 2, Type: 4, IsReal: true},
	{ID: 3, Type: 4, IsReal: true},
	{ID: 4, Type: 4, IsReal: true},
}

var AvailableFakeCards = []Card{
	{ID: 1, Type: 1, IsReal: false},
	{ID: 1, Type: 2, IsReal: false},
	{ID: 1, Type: 3, IsReal: false},
	{ID: 1, Type: 4, IsReal: false},
}
