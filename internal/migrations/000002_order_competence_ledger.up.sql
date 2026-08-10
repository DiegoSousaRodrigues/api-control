ALTER TABLE "order"
    ADD COLUMN order_year SMALLINT,
    ADD COLUMN order_month SMALLINT,
    ADD COLUMN opening_balance NUMERIC(14,2) NOT NULL DEFAULT 0,
    ADD COLUMN previous_month_payment NUMERIC(14,2) NOT NULL DEFAULT 0,
    ADD COLUMN carried_balance NUMERIC(14,2) NOT NULL DEFAULT 0;

ALTER TABLE "order"
    ADD CONSTRAINT order_competence_complete CHECK ((order_year IS NULL) = (order_month IS NULL)),
    ADD CONSTRAINT order_year_valid CHECK (order_year IS NULL OR order_year BETWEEN 1 AND 9999),
    ADD CONSTRAINT order_month_valid CHECK (order_month IS NULL OR order_month BETWEEN 1 AND 12),
    ADD CONSTRAINT order_balances_nonnegative CHECK (opening_balance >= 0 AND previous_month_payment >= 0 AND carried_balance >= 0),
    ADD CONSTRAINT order_payment_within_opening CHECK (previous_month_payment <= opening_balance),
    ADD CONSTRAINT order_carried_balance_consistent CHECK (carried_balance = opening_balance - previous_month_payment);

CREATE UNIQUE INDEX uq_order_client_competence
    ON "order" (client_id, order_year, order_month)
    WHERE order_year IS NOT NULL AND order_month IS NOT NULL;
CREATE INDEX idx_order_competence ON "order" (order_year, order_month, id);

CREATE TABLE client_account_entry (
    id BIGSERIAL PRIMARY KEY,
    date_created TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    client_id BIGINT NOT NULL REFERENCES client(id),
    order_id BIGINT REFERENCES "order"(id),
    entry_type VARCHAR(16) NOT NULL CHECK (entry_type IN ('charge', 'payment')),
    amount NUMERIC(14,2) NOT NULL CHECK (amount > 0),
    order_year SMALLINT NOT NULL CHECK (order_year BETWEEN 1 AND 9999),
    order_month SMALLINT NOT NULL CHECK (order_month BETWEEN 1 AND 12)
);

CREATE UNIQUE INDEX uq_client_account_entry_order_type
    ON client_account_entry (order_id, entry_type) WHERE order_id IS NOT NULL;
CREATE INDEX idx_client_account_entry_client
    ON client_account_entry (client_id, id);
CREATE INDEX idx_client_account_entry_competence
    ON client_account_entry (client_id, order_year, order_month, id);
