package controllers

import "lms/models"

func normalizeReceiptMap(row map[string]interface{}) {
	if len(row) == 0 {
		return
	}

	if value, ok := row["payment_date"]; ok {
		row["payment_date"] = normalizeJakartaDateValue(value)
	}
	if value, ok := row["created_at"]; ok {
		row["created_at"] = normalizeJakartaDateTimeValue(value)
	}
}

func normalizeReceiptMaps(rows []map[string]interface{}) {
	for _, row := range rows {
		normalizeReceiptMap(row)
	}
}

func normalizeReceiptModel(row *models.Receipt) {
	if row == nil {
		return
	}
	if row.PaymentDate != nil {
		converted := reinterpretAsJakartaClock(*row.PaymentDate)
		row.PaymentDate = &converted
	}
	if row.CreatedAt != nil {
		converted := reinterpretAsJakartaClock(*row.CreatedAt)
		row.CreatedAt = &converted
	}
}

func normalizeReceiptModels(rows []models.Receipt) {
	for index := range rows {
		normalizeReceiptModel(&rows[index])
	}
}
