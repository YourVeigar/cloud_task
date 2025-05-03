package model

// UpdateConfigRequest модель для изменения конфига клиента
type UpdateConfigRequest struct {
	ClientId       string `json:"client_id"`
	Capacity       int    `json:"capacity"`
	RefillInterval string `json:"refill_interval"`
}

// DeleteConfigRequest модель для удаления конфига клиента
type DeleteConfigRequest struct {
	ClientId string `json:"client_id"`
}
