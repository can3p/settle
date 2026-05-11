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

### Currency conversion

Is out of scope for the library. You'll need to provide it yourself! Luckily the interface is
very simple

```
type CurrencyConverter interface {
    Convert(a *money.Money, target money.Currency) (*money.Money, error)
}
```

## Licence

MIT
