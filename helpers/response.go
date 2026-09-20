package helpers

type response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
	Meta    interface{} `json:"meta,omitempty"`
}

// Meta adalah informasi paginasi yang menyertai response berbentuk list.
type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func APIResponse(code int, success bool, message string, data interface{}) response {

	return response{
		Success: success,
		Message: message,
		Code:    code,
		Data:    data,
	}

}

// APIResponseWithMeta sama dengan APIResponse tapi menambahkan blok `meta`,
// dipakai endpoint yang mendukung paginasi.
func APIResponseWithMeta(code int, success bool, message string, data interface{}, meta interface{}) response {

	return response{
		Success: success,
		Message: message,
		Code:    code,
		Data:    data,
		Meta:    meta,
	}

}
