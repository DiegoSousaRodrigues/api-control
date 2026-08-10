# Client balance report contract

Status: approved for implementation on 2026-08-10.

## Scope

`Client balance` is a commercial report for the complete order history of one client.

- Profit is `sale total - purchase total`.
- Sale total is the total value of products sold.
- Freight, taxes, fees, payments, previous balances and ledger entries do not affect this report.
- Every currently authenticated user may view purchase cost and profit.
- The monthly balance report is outside this scope and requires a separate specification.

## Historical source

The authoritative grain is one `order_sku` row. New order items must preserve:

```text
snapshot_version
unit_purchase_price
purchase_total
unit_sale_price
price                  # existing historical sale line total
```

For a version 1 item:

```text
purchase_total = unit_purchase_price * quantity
price          = unit_sale_price * quantity
profit_total   = price - purchase_total
```

Profit is derived, not persisted. All persistence and arithmetic use PostgreSQL `NUMERIC` and `decimal.Decimal`.

Changing the current SKU purchase or sale price must never change a historical report.

## Legacy data

Existing items receive `snapshot_version = 0` and nullable purchase snapshots. The current SKU cost must never be used as a historical backfill.

If any item in an aggregate has no reliable purchase snapshot:

- `saleTotal` remains available;
- `purchaseTotal` is `null`;
- `profitTotal` is `null`;
- `costComplete` is `false`;
- `missingCostItemCount` reports the number of incomplete item rows.

A known partial purchase subtotal must never be presented as the complete purchase total.

## Implemented endpoint

```http
GET /report/client-balance?clientId=42
Authorization: Bearer <token>
```

Success response:

```json
{
  "client": {
    "id": 42,
    "name": "Client",
    "active": true
  },
  "totals": {
    "orderCount": 4,
    "quantityTotal": 47,
    "purchaseTotal": 1082.00,
    "saleTotal": 1645.00,
    "profitTotal": 563.00,
    "costComplete": true,
    "missingCostItemCount": 0
  },
  "months": [
    {
      "year": 2026,
      "month": 8,
      "orderCount": 1,
      "quantityTotal": 20,
      "purchaseTotal": 200.00,
      "saleTotal": 300.00,
      "profitTotal": 100.00,
      "costComplete": true,
      "missingCostItemCount": 0
    }
  ]
}
```

Money values are JSON numbers, never localized strings. Months are ordered by descending competence. Legacy orders with no competence are returned as `year: null` and `month: null` rather than silently excluded.

## HTTP behavior

- `200`: existing client, including a client with no orders;
- `400`: missing or malformed `clientId`;
- `401`: missing or invalid authentication;
- `404`: client does not exist;
- `500`: sanitized internal failure.

Inactive clients remain queryable for historical reporting.

## Implementation

- Migration `000003_order_item_cost_snapshots` adds the versioned purchase and sale snapshots without fabricating legacy costs.
- Order creation writes snapshot version 1 in the same transaction as the order, items and account ledger entries.
- `GET /report/client-balance` is protected by the existing JWT middleware and is available to every authenticated user under the approved access rule.
- The repository aggregates directly from `order` and `order_sku`; it does not join the current SKU price or `client_account_entry`.
- The Next.js BFF exposes `GET /api/report/client-balance?clientId=` and the protected UI is available at `/report/client-balance`.
- The UI uses the server aggregates as authoritative values, keeps `clientId` in the URL and renders a table on desktop and monthly cards on small screens.

## Invariants

- Client total equals the sum of its monthly aggregates.
- `profitTotal = saleTotal - purchaseTotal` when `costComplete` is true.
- An order with multiple items is counted once.
- Payments never multiply or modify commercial sale values.
- Pagination or future drill-down endpoints do not change overall totals.
- Common order DTOs do not expose purchase cost.
- Report queries never use the current SKU price and never join the client ledger to calculate profit.

## Required acceptance examples

### Historical snapshot

Given an item sold for 100 with a purchase snapshot of 60  
And the current product purchase price changes to 80  
When the historical report is requested  
Then sale remains 100, purchase remains 60 and profit remains 40.

### Missing cost

Given a legacy item without a reliable purchase snapshot  
When its aggregate is returned  
Then purchase and profit are null  
And the current product cost is not used.

### Commercial and financial separation

Given a July sale paid in August  
When the commercial report is requested  
Then the sale and profit remain assigned to July  
And the August payment does not change those values.

### Reconciliation

Given monthly sales of 100, 200 and 300 for the same client  
When the complete client report is requested  
Then the client sale total is 600.

## Deferred work

- Monthly balance report and its additional business rules;
- cancellation, return and reversal semantics (orders currently have no reliable status model);
- role-based restriction for cost/profit, if access rules change;
- export and detailed order drill-down.
