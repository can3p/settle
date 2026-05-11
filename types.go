package settle

import "github.com/Rhymond/go-money"

type Participant interface {
	ParticipantID() string
}

type Transfer struct {
	From   Participant
	To     Participant
	Amount *money.Money
}

func (t Transfer) Add(m *money.Money) (Transfer, error) {
	sum, err := t.Amount.Add(m)

	if err != nil {
		return Transfer{}, err
	}

	return Transfer{
		From:   t.From,
		To:     t.To,
		Amount: sum,
	}, nil
}

func (t Transfer) Convert(c money.Currency, conv CurrencyConverter) (Transfer, error) {
	// no need to convert the currency to itself
	if c.Code == t.Amount.Currency().Code {
		return t, nil
	}

	amount, err := conv.Convert(t.Amount, c)

	if err != nil {
		return Transfer{}, err
	}

	return Transfer{
		From:   t.From,
		To:     t.To,
		Amount: amount,
	}, nil
}

type SettlementResult []Transfer

type Expense struct {
	payer  Participant
	amount *money.Money
	layout ExpenseLayout
}

type ParticipantExpence struct {
	participant Participant
	amount      *money.Money
}

type ExpenseLayout interface {
	Split(a *money.Money) ([]ParticipantExpence, error)
}

type CurrencyConverter interface {
	Convert(a *money.Money, target money.Currency) (*money.Money, error)
}
