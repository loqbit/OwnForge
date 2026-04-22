# common/errs — Unified Business Errors

Provides layered custom error types that distinguish between the **Msg shown to the frontend** and the **Err used for backend logs**.

## Error Codes

| Constant | Value | Meaning |
|------|-----|------|
| `CodeOK` | 0 | success |
| `CodeParamErr` | 400 | parameter error |
| `CodeUnauthorized` | 401 | unauthorized |
| `CodeForbidden` | 403 | forbidden |
| `CodeNotFound` | 404 | not found |
| `CodeServerErr` | 500 | internal server error |

## Usage

```go
import "github.com/ownforge/ownforge/pkg/errs"

// Parameter error (passed through to the frontend)
return errs.NewParamErr("password length must be at least 6 characters", err)

// system error (hide internal details)
return errs.NewServerErr(err) // msg fixed to "system busy"

// Custom Error Codes
return errs.New(errs.CodeNotFound, "user not found", err)
```

## Handle uniformly in handlers

```go
if customErr, ok := err.(*errs.CustomError); ok {
    c.JSON(customErr.Code, gin.H{"code": customErr.Code, "msg": customErr.Msg})
} else {
    c.JSON(500, gin.H{"code": 500, "msg": "system busy"})
}
```
