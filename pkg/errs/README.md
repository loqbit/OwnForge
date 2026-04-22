# common/errs — 统一业务错误

提供分层的自定义错误类型，区分**给前端看的 Msg**和**给后端查日志的 Err**。

## 错误码

| 常量 | 值 | 含义 |
|------|-----|------|
| `CodeOK` | 0 | 成功 |
| `CodeParamErr` | 400 | 参数错误 |
| `CodeUnauthorized` | 401 | 未授权 |
| `CodeForbidden` | 403 | 禁止访问 |
| `CodeNotFound` | 404 | 未找到 |
| `CodeServerErr` | 500 | 服务器内部错误 |

## 用法

```go
import "github.com/ownforge/ownforge/pkg/errs"

// 参数错误（透传给前端）
return errs.NewParamErr("密码长度不能少于 6 位", err)

// 系统错误（隐藏内部细节）
return errs.NewServerErr(err) // msg 固定为 "系统繁忙"

// 自定义错误码
return errs.New(errs.CodeNotFound, "用户不存在", err)
```

## 在 Handler 中统一处理

```go
if customErr, ok := err.(*errs.CustomError); ok {
    c.JSON(customErr.Code, gin.H{"code": customErr.Code, "msg": customErr.Msg})
} else {
    c.JSON(500, gin.H{"code": 500, "msg": "系统繁忙"})
}
```
