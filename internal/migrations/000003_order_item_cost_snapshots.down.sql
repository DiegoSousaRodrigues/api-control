ALTER TABLE order_sku
    DROP CONSTRAINT IF EXISTS order_sku_sale_total_consistent,
    DROP CONSTRAINT IF EXISTS order_sku_purchase_total_consistent,
    DROP CONSTRAINT IF EXISTS order_sku_unit_sale_positive,
    DROP CONSTRAINT IF EXISTS order_sku_purchase_total_nonnegative,
    DROP CONSTRAINT IF EXISTS order_sku_unit_purchase_nonnegative,
    DROP CONSTRAINT IF EXISTS order_sku_snapshot_complete,
    DROP CONSTRAINT IF EXISTS order_sku_snapshot_version_valid,
    DROP COLUMN IF EXISTS unit_sale_price,
    DROP COLUMN IF EXISTS purchase_total,
    DROP COLUMN IF EXISTS unit_purchase_price,
    DROP COLUMN IF EXISTS snapshot_version;
