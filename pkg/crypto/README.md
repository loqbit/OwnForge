# common/crypto — Password Hashing

Password hashing and verification based on bcrypt.

## Usage

```go
import "github.com/ownforge/ownforge/pkg/crypto"

// At registration: plaintext -> hash
hashed, err := crypto.HashPassword("my-secret-password")

// At login: plaintext vs hash
ok := crypto.CheckPasswordHash("my-secret-password", hashed) // true
```
