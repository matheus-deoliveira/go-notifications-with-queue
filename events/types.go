package events

import "time"

// CreditApprovedEvent será enviada para a fila
type CreditApprovedEvent struct {
	EventId       string    `json:"event_id"`
	ProposalId    string    `json:"proposal_id"`
	ApprovedAt    time.Time `json:"approved_at"`
	CustomerName  string    `json:"customer_name"`
	CustomerEmail string    `json:"customer_email"`
	CreditLimit   float64   `json:"credit_limit"`
}
