package utils

import (
	"strconv"
	"strings"
)

// CurrencyToFloat64 converte um valor de moeda (ex: "R$ 12,50") para float64
func CurrencyToFloat64(currency string) (float64, error) {
	// Remove o símbolo de moeda e espaços
	cleaned := strings.ReplaceAll(currency, "R$", "")
	cleaned = strings.TrimSpace(cleaned)
	// Substitui a vírgula por ponto
	cleaned = strings.ReplaceAll(cleaned, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", ".")
	// Converte para float64
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// Float64ToCurrency converte um valor float64 para moeda (ex: 12.50 para "R$ 12,50")
func Float64ToCurrency(value float64) string {
	// Converte o valor para string com duas casas decimais
	strValue := strconv.FormatFloat(value, 'f', 2, 64)
	// Substitui o ponto por vírgula
	strValue = strings.ReplaceAll(strValue, ".", ",")
	// Adiciona o símbolo de moeda
	return "R$ " + strValue
}