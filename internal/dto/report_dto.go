package dto

type ReportClientDTO struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type ReportBalanceTotalsDTO struct {
	OrderCount           int64  `json:"orderCount"`
	QuantityTotal        int64  `json:"quantityTotal"`
	PurchaseTotal        *Money `json:"purchaseTotal"`
	SaleTotal            Money  `json:"saleTotal"`
	ProfitTotal          *Money `json:"profitTotal"`
	CostComplete         bool   `json:"costComplete"`
	MissingCostItemCount int64  `json:"missingCostItemCount"`
}

type ClientBalanceMonthDTO struct {
	Year  *int16 `json:"year"`
	Month *int16 `json:"month"`
	ReportBalanceTotalsDTO
}

type ClientBalanceReportDTO struct {
	Client ReportClientDTO         `json:"client"`
	Totals ReportBalanceTotalsDTO  `json:"totals"`
	Months []ClientBalanceMonthDTO `json:"months"`
}
