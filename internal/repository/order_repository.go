package repository

import (
	"errors"
	"fmt"

	"github.com/api-control/internal/domain"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var OrderRepository IOrderRepository = &orderRepository{db: domain.BaseRepository{}}

type IOrderRepository interface {
	List(year int16, month int16) (entity *[]domain.Order, err error)
	Add(entity domain.Order) (err error)
	OpenBalance(clientID int64, year int16, month int16) (decimal.Decimal, error)
	FindByID(id string) (entity *domain.Order, err error)
	Update(id int64, entity domain.Order) (err error)
}

type orderDatabase interface {
	PSQL() *gorm.DB
}

type orderRepository struct {
	db orderDatabase
}

var (
	ErrOrderClientInactive             = errors.New("order client is inactive")
	ErrOrderSkuInactive                = errors.New("order sku is inactive")
	ErrOrderSkuPurchasePriceMissing    = errors.New("order sku purchase price is required")
	ErrOrderSkuSnapshotOutOfRange      = errors.New("order sku snapshot exceeds numeric range")
	ErrOrderPaymentExceedsBalance      = errors.New("previous month payment exceeds accumulated balance")
	ErrOrderFinancialUpdateUnsupported = errors.New("financial order updates are not supported")
	ErrOrderCompetenceExists           = errors.New("client already has an order for this competence")
	ErrOrderRetroactiveCompetence      = errors.New("cannot create an order before an existing later competence")
	ErrOrderPaymentNegative            = errors.New("previous month payment cannot be negative")
	ErrOrderCompetenceRequired         = errors.New("order competence is required")
)

func (c *orderRepository) List(year int16, month int16) (entity *[]domain.Order, err error) {
	db := c.db.PSQL()

	if err := db.Where("order_year = ? AND order_month = ?", year, month).Order("id").Preload("Client").Preload("OrderSkus").Preload("OrderSkus.Sku").Find(&entity); err.Error != nil {
		return nil, err.Error
	}

	if entity == nil {
		return nil, err
	}

	return entity, nil
}

func (c *orderRepository) Add(order domain.Order) (err error) {
	if order.OrderYear == nil || order.OrderMonth == nil {
		return ErrOrderCompetenceRequired
	}
	db := c.db.PSQL()

	return db.Transaction(func(tx *gorm.DB) error {
		if err := validateOrderClientForUpdate(tx, order.ClientId); err != nil {
			return err
		}
		if err := hydrateOrderSkuSnapshots(tx, &order); err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&domain.Order{}).Where("client_id = ? AND order_year = ? AND order_month = ?", order.ClientId, *order.OrderYear, *order.OrderMonth).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrOrderCompetenceExists
		}
		var later int64
		if err := tx.Model(&domain.Order{}).Where("client_id = ? AND (order_year > ? OR (order_year = ? AND order_month > ?))", order.ClientId, *order.OrderYear, *order.OrderYear, *order.OrderMonth).Count(&later).Error; err != nil {
			return err
		}
		if later > 0 {
			return ErrOrderRetroactiveCompetence
		}
		openingBalance, err := queryOpenBalance(tx, order.ClientId, *order.OrderYear, *order.OrderMonth)
		if err != nil {
			return err
		}
		order.OpeningBalance = openingBalance
		carriedBalance, err := calculateCarriedBalance(openingBalance, order.PreviousMonthPayment)
		if err != nil {
			return err
		}
		order.CarriedBalance = carriedBalance
		total := orderItemsTotal(order.OrderSkus)
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		entries := []domain.ClientAccountEntry{{ClientID: order.ClientId, OrderID: &order.ID, EntryType: domain.AccountEntryCharge, Amount: total, OrderYear: *order.OrderYear, OrderMonth: *order.OrderMonth}}
		if order.PreviousMonthPayment.IsPositive() {
			paymentYear, paymentMonth := previousCompetence(*order.OrderYear, *order.OrderMonth)
			entries = append(entries, domain.ClientAccountEntry{ClientID: order.ClientId, OrderID: &order.ID, EntryType: domain.AccountEntryPayment, Amount: order.PreviousMonthPayment, OrderYear: paymentYear, OrderMonth: paymentMonth})
		}
		return tx.Create(&entries).Error
	})
}

func (c *orderRepository) OpenBalance(clientID int64, year int16, month int16) (decimal.Decimal, error) {
	db := c.db.PSQL()
	if err := validateOrderClient(db, clientID); err != nil {
		return decimal.Zero, err
	}
	return queryOpenBalance(db, clientID, year, month)
}

func queryOpenBalance(db *gorm.DB, clientID int64, year int16, month int16) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := db.Model(&domain.ClientAccountEntry{}).Select("COALESCE(SUM(CASE WHEN entry_type = ? THEN amount ELSE -amount END), 0)", domain.AccountEntryCharge).Where("client_id = ? AND (order_year < ? OR (order_year = ? AND order_month < ?))", clientID, year, year, month).Scan(&balance).Error
	return balance, err
}

func previousCompetence(year int16, month int16) (int16, int16) {
	if month == 1 {
		return year - 1, 12
	}
	return year, month - 1
}

func calculateCarriedBalance(opening decimal.Decimal, payment decimal.Decimal) (decimal.Decimal, error) {
	if payment.IsNegative() {
		return decimal.Zero, ErrOrderPaymentNegative
	}
	if payment.GreaterThan(opening) {
		return decimal.Zero, ErrOrderPaymentExceedsBalance
	}
	return opening.Sub(payment), nil
}

func (c *orderRepository) FindByID(id string) (entity *domain.Order, err error) {
	db := c.db.PSQL()
	var order domain.Order

	if err := db.Where("id = ?", id).Preload("Client").Preload("OrderSkus").Preload("OrderSkus.Sku").First(&order); err.Error != nil {
		return nil, err.Error
	}

	return &order, nil
}

func (c *orderRepository) Update(id int64, entity domain.Order) (err error) {
	return ErrOrderFinancialUpdateUnsupported
}

func hydrateOrderSkuSnapshots(tx *gorm.DB, order *domain.Order) error {
	for index := range order.OrderSkus {
		var sku domain.Sku
		if err := tx.Where("id = ?", order.OrderSkus[index].SkuID).First(&sku).Error; err != nil {
			return err
		}
		if !sku.Active {
			return fmt.Errorf("%w: %d", ErrOrderSkuInactive, sku.ID)
		}

		if err := applyOrderSkuSnapshot(&order.OrderSkus[index], sku); err != nil {
			return fmt.Errorf("hydrate sku %d snapshot: %w", sku.ID, err)
		}
	}

	return nil
}

func validateOrderClient(tx *gorm.DB, clientID int64) error {
	var client domain.Client
	if err := tx.Where("id = ?", clientID).First(&client).Error; err != nil {
		return err
	}
	if !client.Active {
		return fmt.Errorf("%w: %d", ErrOrderClientInactive, client.ID)
	}

	return nil
}

func validateOrderClientForUpdate(tx *gorm.DB, clientID int64) error {
	var client domain.Client
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", clientID).First(&client).Error; err != nil {
		return err
	}
	if !client.Active {
		return fmt.Errorf("%w: %d", ErrOrderClientInactive, client.ID)
	}
	return nil
}

func applyOrderSkuSnapshot(orderSku *domain.OrderSku, sku domain.Sku) error {
	if sku.PurchasePrice == nil {
		return ErrOrderSkuPurchasePriceMissing
	}

	quantity := decimal.NewFromInt(int64(orderSku.Quantity))
	unitPurchasePrice := sku.PurchasePrice.Copy()
	purchaseTotal := unitPurchasePrice.Mul(quantity)
	unitSalePrice := sku.SalePrice.Copy()
	saleTotal := unitSalePrice.Mul(quantity)
	if !fitsNumeric14_2(purchaseTotal) || !fitsNumeric14_2(saleTotal) {
		return ErrOrderSkuSnapshotOutOfRange
	}

	orderSku.Name = sku.Name
	orderSku.SnapshotVersion = 1
	orderSku.UnitPurchasePrice = decimalPointer(unitPurchasePrice)
	orderSku.PurchaseTotal = decimalPointer(purchaseTotal)
	orderSku.UnitSalePrice = decimalPointer(unitSalePrice)
	orderSku.Price = saleTotal

	return nil
}

func decimalPointer(value decimal.Decimal) *decimal.Decimal {
	copy := value.Copy()
	return &copy
}

func orderItemsTotal(items []domain.OrderSku) decimal.Decimal {
	total := decimal.Zero
	for _, item := range items {
		total = total.Add(item.Price)
	}
	return total
}

func fitsNumeric14_2(value decimal.Decimal) bool {
	max := decimal.RequireFromString("999999999999.99")
	return value.Abs().LessThanOrEqual(max)
}
