package errs

// Common business status codes
const (
	Success      = 200 // success
	ServerErr    = 500 // internal server error
	ParamErr     = 400 // parameter error
	Unauthorized = 401 // not signed in
	Forbidden    = 403 // forbidden
	NotFound     = 404 // resource not found
	Gone         = 410 // resource expired or deleted
)
