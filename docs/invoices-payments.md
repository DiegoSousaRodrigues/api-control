# Invoices and independent payments architecture

Status: approved for phased implementation on 2026-08-11.

The detailed cross-stack specification is maintained in the Control workspace as
`FATURAS_PAGAMENTOS_SPEC_FASEAMENTO.md`. This document freezes the decisions that
the API implementation must preserve.

## Domain decisions

- `Pedido` becomes `Fatura` in the product and `invoice` in code and storage.
- There is at most one active invoice per client and billing period.
- An invoice is created once for the month and contains at least one product.
- Product quantities are positive integers.
- Payments are independent events and a client may have multiple payments.
- A payment is accepted when the account has debt, is settled, or already has credit.
- Payments are allocated automatically to the oldest open invoices using FIFO.
- Any unallocated payment remains client credit.
- Existing credit is allocated automatically when a new invoice is issued.
- Credit can only settle current or future invoices; refunds and transfers are out of scope.
- Retroactive payment effective dates are supported, but future dates are rejected.
- Invoices and payments are never deleted. Corrections use cancellation or reversal.
- Business and financial tables do not reference `users`; authentication remains isolated.
- Payment method, external reference, and a server idempotency key are intentionally outside the initial schema.
- The current development database may be discarded completely. No ETL is required.

## Authoritative formulas

```text
netBalance = invoice charges
           - invoice cancellation credits
           - posted payments
           + payment reversal debits

netBalance > 0 => debt
netBalance = 0 => settled
netBalance < 0 => credit
```

Invoices and payments are authoritative. Allocation rows explain invoice settlement
but never change the account balance a second time.

```text
invoiceOpenAmount = 0, when the invoice is canceled
                  or invoice.productsTotal - active invoice allocations

paymentUnallocatedAmount = 0, when the payment is reversed
                         or payment.amount - active payment allocations
```

## Target schema

The clean baseline contains plural, non-reserved table names:

```text
users
clients
products
billing_periods
invoices
invoice_items
payments
payment_allocations
schema_migrations
```

Conventions:

- identity `BIGINT` primary keys;
- `TIMESTAMPTZ` instants and `DATE` civil/effective dates;
- monthly competence stored as the first day of the month;
- unit prices in `NUMERIC(14,2)`;
- totals and balances in `NUMERIC(15,2)`, safe for cent-based JavaScript numbers;
- positive integer quantities;
- financial foreign keys use `ON DELETE RESTRICT`;
- JSON money values are numbers and Go uses `decimal.Decimal`.

The model removes the legacy `price float64`, `snapshot_version`,
`previous_month_payment`, `carried_balance`, and `client_account_entry` concepts.

## Transaction rules

Invoice issue, invoice cancellation, payment posting, and payment reversal must:

1. start one transaction;
2. lock the client row with `FOR UPDATE`;
3. recompute invoices, payments, and allocations inside that transaction;
4. validate allocation conservation;
5. commit all effects atomically;
6. return the authoritative resulting account position.

An invoice snapshots product name, purchase price, sale price, and generated totals.
Current product prices never affect historical reports.

An invoice from a past competence is accepted only when the client has no later active
invoice. Only the client's latest active invoice may be canceled. These rules prevent
later invoice snapshots from becoming inconsistent.

Payment allocation represents the current settlement view. A retroactive payment may
settle any invoice that is currently open. Financial history is recalculated using all
currently known retroactive events.

## Cutover rule

The existing embedded migrations cannot create an empty database and the new baseline
is incompatible with the legacy runtime. Therefore:

- phases 1 through 7 are committed and pushed incrementally to the default branches,
  but incompatible components remain staged/inactive and are tested against a
  separate, disposable PostgreSQL database;
- the new baseline is not activated in the normal migration runner early;
- legacy routes and domains remain until the coordinated cutover;
- baseline activation, legacy removal, database reset, API switch, front switch, and
  report switch happen together in phase 8;
- no destructive database operation is performed before the exact development target
  is resolved and checked again.

## Planned phases

0. Freeze specification and reset checklist.
1. Clean PostgreSQL baseline, new domains, authentication/client/product CRUD staging.
2. Account position engine and FIFO allocation.
3. Invoice API.
4. Payment, account, and statement APIs.
5. Payment/account BFF and UI.
6. Rename and migrate Order UI to Invoice.
7. Migrate the client commercial report.
8. Controlled database reset and coordinated cutover.
9. Specify and implement the monthly report on the stable foundation.

Every phase requires an independent regression review, applicable automated gates,
an intentional commit, and a push before the next phase begins.

## Accepted risks

- No server-side idempotency key in the first version. The front disables duplicate
  submission and automatic mutation retry, but a manual retry after an ambiguous
  timeout can still duplicate a financial event.
- No actor reference on business or financial events.
- Retroactive payments can change historical financial reporting.
- The development reset destroys all current data.
- Credit cannot be refunded or transferred.

## Reset checklist

The reset is authorized in principle but remains a separate destructive action:

1. stop API and front processes;
2. resolve the database target without printing credentials;
3. verify the exact database/schema is the disposable development target;
4. replace the embedded legacy migration chain with the approved clean baseline;
5. recreate only that verified target;
6. run the migration runner;
7. create the initial authentication user through a safe administrative command;
8. run client, product, payment, credit, invoice, account, and report smoke tests;
9. start the new front only after the new API and schema are ready.

The phase 1 bootstrap command has this frozen interface:

```powershell
go run ./cmd/control-admin create-user --login <login>
```

It reads and confirms the password interactively without terminal echo, rejects a
password supplied as an argument, uses `DB_CONNECTION_STRING` only for the already
verified target, never prints credentials, hashes, or the connection string, and
fails without mutation when the login already exists. It creates only the initial
authentication user and no business data. The reset cannot pass the bootstrap step
until this command exists and has been validated in phase 1.
