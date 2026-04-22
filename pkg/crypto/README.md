# common/crypto — 密码哈希

基于 bcrypt 的密码加密与校验。

## 用法

```go
import "github.com/ownforge/ownforge/pkg/crypto"

// 注册时：明文 → 哈希
hashed, err := crypto.HashPassword("my-secret-password")

// 登录时：明文 vs 哈希
ok := crypto.CheckPasswordHash("my-secret-password", hashed) // true
```
