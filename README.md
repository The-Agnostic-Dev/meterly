# Meterly

Meterly is the prepaid electricity and service charge desk for **Palmgrove Court Estate, Lekki**. The facility
manager, the estate accountant and residents use it to:

- see every flat, its household and its prepaid meter;
- buy units (top-ups): a naira amount is turned into kWh at the flat's tariff band and an STS token is issued;
- record meter readings and see monthly consumption per flat;
- bill and collect the monthly service charge;
- review estate-wide usage per month and block;
- look up the history of every token sold.

It is a small Go service with server-rendered pages (`html/template`) and a little vanilla JavaScript. Data lives in
memory and is seeded with demo data every time the app starts, with dates relative to today.

## Running it

Requirements: Go 1.24 (or Docker).

```sh
go run ./cmd/meterly          # http://localhost:5120
```

or

```sh
docker compose up --build     # http://localhost:5120
```

Environment variables (see `.env.example`):

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `5120` | HTTP port |
| `METERLY_VEND_DELAY` | `400ms` | Simulated round trip to the token vending gateway |
| `METERLY_DEBUG` | unset | Log every request |

Restarting the app resets all data to the demo seed.

## Demo logins

All demo accounts use the password `meterly2026`.

| Who | Email | Role |
| --- | --- | --- |
| Tunde Bakare | `fm@palmgrovecourt.ng` | Facility manager: everything, including editing residents and readings |
| Ngozi Eze | `accounts@palmgrovecourt.ng` | Estate accountant: reports, token history, service charge payments |
| Chioma Okafor | `chioma.okafor@gmail.com` | Resident of flat A2: own meter, own flat, buy units |
| Ibrahim Musa | `ibrahim.musa@gmail.com` | Resident of flat B4 |

## Tests

```sh
go test ./...
```

## How the numbers work

These rules come from the estate's agreement with the DisCo and the residents' AGM. Please keep them in mind before
"fixing" a figure that looks odd.

**Buying units**

- A top-up must be between ₦1,000 and ₦500,000.
- The amount a resident pays **includes 7.5% VAT**. Meterly takes the VAT out first
  (VAT = amount × 7.5 / 107.5), and the rest is the energy value. For ₦10,000 the receipt shows
  ₦697.67 VAT and ₦9,302.33 energy value.
- Units = energy value ÷ the flat's tariff band rate, rounded down to 0.01 kWh.
- Tariff bands follow the feeder that supplies each block (rates per kWh, before VAT):

  | Band | Rate | Supply | Flats |
  | --- | --- | --- | --- |
  | A | ₦209.50 | at least 20 hours a day | Block A, penthouses C5 and C6 |
  | B | ₦63.30 | at least 16 hours a day | Block B, C1 to C4 |
  | C | ₦50.30 | at least 12 hours a day | none at the moment |

  So the same ₦10,000 gives 44.40 kWh on Band A and 146.95 kWh on Band B.
- Each buy form carries a one-time request id, so a form submitted twice returns the first receipt instead of
  charging again.
- Meters with less than 20 kWh left are listed on the dashboard as low balance.

**Readings and consumption**

- Readings are the cumulative kWh register on the meter, taken by the facility manager (seeded on the 28th of each
  month and today).
- A month's consumption is the last reading in that month minus the last reading before the month started. A month
  with no reading, or with no earlier reading, shows "No reading".
- New readings cannot be dated in the future or be lower than the meter's latest reading. Corrections must stay
  between the readings before and after.

**Service charge**

- Monthly service charge by flat type: 1-bed ₦25,000, 2-bed ₦35,000, 3-bed ₦45,000, 4-bed penthouse ₦60,000.
- Invoices are raised on the 1st and due on the 10th. After the 10th an unpaid or part-paid invoice shows as
  **overdue**.
- A payment cannot be more than the invoice balance; extra money is recorded as credit on the next month's invoice.

**Estate usage report**

- Shows, per flat, consumption from readings plus tokens and units bought in the month.
- Click a column heading to sort.
- The report remembers the month and block you last picked, so it opens the same way next time.

**Token history** lists every token sold, newest first, 10 per page, and can be filtered by flat.

All times are Lagos time (WAT). Money is shown like ₦12,500.00.

## Project layout

```
cmd/meterly/         main: config, seed, HTTP server
internal/clock/      Lagos time helpers
internal/money/      naira amounts (kobo) and VAT
internal/energy/     kWh quantities (hundredths)
internal/store/      in-memory store and models
internal/seed/       demo estate
internal/meters/     buying units, token vendor
internal/readings/   meter readings and monthly consumption
internal/billing/    service charge invoices and payments
internal/report/     estate usage report
internal/prefs/      remembered screen choices
internal/auth/       sign-in sessions
internal/web/        handlers, forms, paging
ui/templates/        HTML templates
ui/static/           CSS and JavaScript
```
