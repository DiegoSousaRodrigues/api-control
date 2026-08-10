ALTER TABLE order_sku
    ADD COLUMN snapshot_version SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN unit_purchase_price NUMERIC(14,2),
    ADD COLUMN purchase_total NUMERIC(14,2),
    ADD COLUMN unit_sale_price NUMERIC(14,2);

ALTER TABLE order_sku
    ADD CONSTRAINT order_sku_snapshot_version_valid
        CHECK (snapshot_version IN (0, 1)),
    ADD CONSTRAINT order_sku_snapshot_complete
        CHECK (
            snapshot_version = 0
            OR (
                unit_purchase_price IS NOT NULL
                AND purchase_total IS NOT NULL
                AND unit_sale_price IS NOT NULL
            )
        ),
    ADD CONSTRAINT order_sku_unit_purchase_nonnegative
        CHECK (unit_purchase_price IS NULL OR unit_purchase_price >= 0),
    ADD CONSTRAINT order_sku_purchase_total_nonnegative
        CHECK (purchase_total IS NULL OR purchase_total >= 0),
    ADD CONSTRAINT order_sku_unit_sale_positive
        CHECK (unit_sale_price IS NULL OR unit_sale_price > 0),
    ADD CONSTRAINT order_sku_purchase_total_consistent
        CHECK (
            snapshot_version = 0
            OR purchase_total = unit_purchase_price * quantity
        ),
    ADD CONSTRAINT order_sku_sale_total_consistent
        CHECK (
            snapshot_version = 0
            OR price = unit_sale_price * quantity
        );
