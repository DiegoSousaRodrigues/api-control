BEGIN;

ALTER TABLE sku
    ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(14,2),
    ADD COLUMN IF NOT EXISTS sale_price NUMERIC(14,2);

-- The legacy price is the existing sale price. Purchase price deliberately
-- remains NULL for legacy rows until the business supplies a real value.
UPDATE sku
SET sale_price = ROUND(price::numeric, 2)
WHERE sale_price IS NULL;

ALTER TABLE sku ALTER COLUMN sale_price SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sku_purchase_price_nonnegative') THEN
        ALTER TABLE sku ADD CONSTRAINT sku_purchase_price_nonnegative
            CHECK (purchase_price IS NULL OR purchase_price >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sku_sale_price_nonnegative') THEN
        ALTER TABLE sku ADD CONSTRAINT sku_sale_price_nonnegative
            CHECK (sale_price > 0) NOT VALID;
    END IF;
END $$;

ALTER TABLE sku VALIDATE CONSTRAINT sku_purchase_price_nonnegative;
ALTER TABLE sku VALIDATE CONSTRAINT sku_sale_price_nonnegative;

-- order_sku.price remains the historical line total; only its storage type
-- changes so order arithmetic no longer mixes decimal and floating point.
ALTER TABLE order_sku
    ALTER COLUMN price TYPE NUMERIC(14,2)
    USING ROUND(price::numeric, 2);

COMMIT;
