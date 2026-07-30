package helpers

type response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Code    int         `json:"code"`
	Data    interface{} `json:"data"`
}

func APIResponse(code int, success bool, message string, data interface{}) response {

	return response{
		Success: success,
		Message: message,
		Code:    code,
		Data:    data,
	}

}
