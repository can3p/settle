# Settle

This is a small package that allows to settle common expenses between a group of people.
If you used an app like splitwise, that's their secret sauce on top of crud app to
record the data. What do we have here?

## Features

- Stable split. The library is designed to always provide the same result given the same input.
- Multi-currency support. Maybe you don't need it, that's fine.

## Concepts

### Settlement

Settlement is a process of deciding who owes what to whom. Settlement is done in one currency
based on a number of expenses.

Settlement result is represented as an array of transfers:

```
type Transfer struct {
    From   Participant
    To     Participant
    Amount *money.Money
}

type SettlementResult []Transfer
```

After the settlement all the members should have paid each other their fair share.

Particular example:

- John pays for him, Bill, Harry and Marry 100 EUR in a restaurant
- Bill pays for John and Marry 100 EUR in a shop

The first expense results in:

Bill -> John 25 EUR
Harry -> John 25 EUR
Marry -> John 25 EUR

The second expense results in:

John -> Bill 50 EUR
Marry -> Bill 50 EUR

The settlement would be:

John -> Bill 25 EUR
Marry -> Bill 50 EUR
Marry -> John 25 EUR
Harry -> John 25 EUR

### Participant

Participant is just an interface that represents a member of a group.

```
type Participant interface {
    ParticipantID() string
}
```

### Expense

Expense is a bill that was paid for one or more members of the group by one of the members
of the group. Any expense has an amount.

#### Amount

Amount is an actual money value in calculation. The amount is expressed in a minimal transferrable
value (i.e. cents for EUR/USD). This library is not meant to be used for accounting and that's
enough precision for all practical purposes.

12.31 EUR will be represented as 1231 internally. We're using [go-money](https://github.com/Rhymond/go-money) for that.

### Rounding

It's really hard to transfer 0.3 cents, it is. However, like it is with accounting one
can chose where to do the rounding. For the simplicity sake the rounding is done
per every expense separately. We're dealing with rounding errors there and someone should
get the fractions.

Here are the rules:

- Every person's part is truncated down to the minimal transferable amount.
- First participant in the layout gets the remainder (the cents lost to truncation).
- This ensures the total always adds up exactly to the expense amount.

### Expense Layout

This is where things get interesting. There are many ways to split up the bill. To keep things
simple in the beginning, here are few options:

- Split in parts for 2..N participants.
  - John and Marry decided to split the bill for the dinner, John pays for one more guest.
- Split equally between 2..N participants. This is the previous case in disguise, everybody has
  one part.
  - John, Harry and Marry pay for the dinner together
- Split in percentages. For a group of people I want to cover 39.13% of the bill, you want to
  cover 17.21% of the bill, another person will field the rest. The rest should be expressed
  like this since someone should eat up the rounding errors.
- Split in absolute values. John has spent 5 USD, Harry spent the rest
- A mix of absolute and proportional layouts.
  - John has spent 5 USD, Harry and Marry pay the rest.
  - John has spent 5 USD, Harry pays for 37%, Marry pays the rest

Of course the original payer may or may not be amongst the participant. Poor Bill, he always pays.

### Settlement calculation

* For every expense the transfers are calculated
* Every transfer is then converted to the target currency
* All transfers are added up
* The end list is always sorted to give the same output

### Post-settlement optimization

By default the library produces one transfer per netted debt between each pair
of participants. A post-settlement optimization can reduce the **number of
transactions** while preserving every participant's net balance.

The library supports this through the functional-options pattern. Pass
`WithOptimizer` with any `Optimizer` function:

```go
s := settle.NewSettlement(eur, conv, expenses...).
    WithOptions(settle.WithOptimizer(settle.GreedyOptimizer))

result, err := s.Settle()
```

A custom optimizer is any function matching the `Optimizer` signature:

```go
func myOptimizer(result settle.SettlementResult) (settle.SettlementResult, error) {
    // ... reduce transactions ...
    return optimized, nil
}

s := settle.NewSettlement(eur, conv, expenses...).
    WithOptions(settle.WithOptimizer(myOptimizer))
```

#### Built-in: GreedyOptimizer

`GreedyOptimizer` greedily matches the participant who is owed the most with
the participant who owes the most. This is the same approach described in
articles examining the Splitwise algorithm, e.g.
[How Does the Splitwise Algorithm Work?](https://medium.com/@howoftech/how-does-the-splitwise-algorithm-work-dc1de5eaa371).

The algorithm works in two phases:

1. **Compute net balances** — for each participant, sum up how much they are
   owed (positive) and how much they owe (negative).
2. **Greedy matching** — sort participants by balance (debtors first,
   creditors last) and use a two-pointer technique: the leftmost entry is the
   max debtor and the rightmost is the max creditor. Settle the smaller of the
   two balances between them, then advance the pointer of whichever participant
   was zeroed out. Entries that cross to the wrong side are skipped. This
   zeroes out at least one participant per iteration, so the result has at most
   *n*−1 transfers for *n* participants with non-zero balances.

The optimizer is applied **after** the raw settlement is computed and **before**
the final sort, so the output remains deterministic. It is also carried over
through `AddExpenses` and `AddRepayments`:

```go
s := settle.NewSettlement(eur, conv).
    WithOptions(settle.WithOptimizer(settle.GreedyOptimizer)).
    AddExpenses(expenses...)

result, err := s.Settle()
```

> **Note:** The greedy algorithm does not always produce the absolute minimum
> number of transactions (that problem is NP-hard in general), but it gives a
> good approximation.

### Currency conversion

The library requires a `CurrencyConverter` implementation:

```
type CurrencyConverter interface {
    Convert(a *money.Money, target money.Currency) (*money.Money, error)
}
```

A built-in `staticConverter` is provided for fixed exchange rates. Rates are expressed
in **full currency units** (the way you'd see them on a market ticker). The converter
automatically handles the difference in fractional digits between currencies.

#### Understanding the subunit problem

go-money stores amounts in the smallest currency unit (subunits):
- **USD**: stored in cents, so $100 = `10000`
- **JPY**: has no subunits (Fraction=0), so ¥15000 = `15000`

Exchange rates are normally quoted in full units: "1 USD = 150 JPY". The static converter
bridges the gap — you pass the standard rate and it adjusts for differing subunit scales
internally.

#### Example: USD to JPY

```go
conv := settle.NewStaticConverter()
// Standard market rate: 1 USD = 150 JPY
conv.AddRate(money.USD, money.JPY, decimal.NewFromInt(150))

// $100 is stored as 10000 (cents). The converter:
// 1. Multiplies subunits by rate: 10000 × 150 = 1500000
// 2. Adjusts for fraction difference (USD=2, JPY=0): 1500000 / 10^2 = 15000
// Result: ¥15000
result, _ := conv.Convert(money.New(100_00, money.USD), *money.GetCurrency(money.JPY))
// result.Amount() == 15000

// The inverse rate is registered automatically:
// ¥15000 back to USD -> $100.00 (10000 cents)
back, _ := conv.Convert(money.New(15000, money.JPY), *money.GetCurrency(money.USD))
// back.Amount() == 10000
```

#### Example: EUR to USD (same subunit scale)

```go
conv := settle.NewStaticConverter()
conv.AddRate(money.EUR, money.USD, decimal.NewFromFloat(1.1))

// €50 (5000 cents) -> $55 (5500 cents). Same Fraction=2, no scaling needed.
result, _ := conv.Convert(money.New(50_00, money.EUR), *money.GetCurrency(money.USD))
// result.Amount() == 5500
```

For more complex or live rates, implement the `CurrencyConverter` interface yourself.

## Basic Usage

```go
package main

import (
    "fmt"

    "github.com/Rhymond/go-money"
    "github.com/can3p/settle"
)

// Implement the CurrencyConverter interface (no-op for single currency)
type NoopConverter struct{}

func (NoopConverter) Convert(a *money.Money, target money.Currency) (*money.Money, error) {
    return money.New(a.Amount(), target.Code), nil
}

func main() {
    john := settle.StringParticipant("john")
    bill := settle.StringParticipant("bill")
    harry := settle.StringParticipant("harry")
    marry := settle.StringParticipant("marry")

    eur := *money.GetCurrency(money.EUR)

    // Create a settlement with initial expenses
    s := settle.NewSettlement(eur, NoopConverter{},
        settle.NewExpense(john, money.New(100_00, money.EUR), settle.NewEvenLayout(john, bill, harry, marry)),
    )

    // Add more expenses later
    s = s.AddExpenses(
        settle.NewExpense(bill, money.New(100_00, money.EUR), settle.NewEvenLayout(john, marry)),
    )

    // Account for transfers that already happened (as reverse obligations)
    // e.g. Harry already paid John 25 EUR
    s = s.AddRepayments(settle.Transfer{
        From:   john,
        To:     harry,
        Amount: money.New(25_00, money.EUR),
    })

    result, err := s.Settle()
    if err != nil {
        panic(err)
    }

    for _, t := range result {
        fmt.Printf("%s -> %s: %s\n", t.From.ParticipantID(), t.To.ParticipantID(), t.Amount.Display())
    }
}
```

Key points:

- Amounts are in minimal currency units (cents): 100 EUR = `10000`
- `AddExpenses` appends expenses to an existing settlement
- `AddRepayments` records already-completed payments — `Transfer{From: bob, To: alice, Amount: 30}`
  means "Bob already sent Alice 30", which reduces Bob's debt accordingly
- Both methods return a new `Settlement` value without mutating the original
- Pass `WithOptions(settle.WithOptimizer(settle.GreedyOptimizer))` to reduce the number of transactions
  via greedy creditor/debtor matching (see [Post-settlement optimization](#post-settlement-optimization))

## Licence

MIT
