package crypto

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt 对原始密码进行哈希。
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash 比较原始密码与哈希值是否匹配。
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
