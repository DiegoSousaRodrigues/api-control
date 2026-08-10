BEGIN;

ALTER TABLE order_sku
    ALTER COLUMN price TYPE DOUBLE PRECISION
    USING price::double precision;

ALTER TABLE sku DROP CONSTRAINT IF EXISTS sku_purchase_price_nonnegative;
ALTER TABLE sku DROP CONSTRAINT IF EXISTS sku_sale_price_nonnegative;
ALTER TABLE sku DROP COLUMN IF EXISTS purchase_price;
ALTER TABLE sku DROP COLUMN IF EXISTS sale_price;

COMMIT;
