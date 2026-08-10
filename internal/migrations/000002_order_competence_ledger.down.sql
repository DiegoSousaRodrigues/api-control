BEGIN;

DROP TABLE IF EXISTS client_account_entry;
DROP INDEX IF EXISTS idx_order_competence;
DROP INDEX IF EXISTS uq_order_client_competence;

ALTER TABLE "order"
    DROP CONSTRAINT IF EXISTS order_carried_balance_consistent,
    DROP CONSTRAINT IF EXISTS order_payment_within_opening,
    DROP CONSTRAINT IF EXISTS order_balances_nonnegative,
    DROP CONSTRAINT IF EXISTS order_month_valid,
    DROP CONSTRAINT IF EXISTS order_year_valid,
    DROP CONSTRAINT IF EXISTS order_competence_complete,
    DROP COLUMN IF EXISTS carried_balance,
    DROP COLUMN IF EXISTS previous_month_payment,
    DROP COLUMN IF EXISTS opening_balance,
    DROP COLUMN IF EXISTS order_month,
    DROP COLUMN IF EXISTS order_year;

COMMIT;
