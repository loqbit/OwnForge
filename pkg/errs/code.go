package errs

// 常用业务状态码
const (
	Success      = 200 // 成功
	ServerErr    = 500 // 服务器内部错误
	ParamErr     = 400 // 参数错误
	Unauthorized = 401 // 未登录
	Forbidden    = 403 // 无权限
	NotFound     = 404 // 资源不存在
	Gone         = 410 // 资源已过期/被删除
)
