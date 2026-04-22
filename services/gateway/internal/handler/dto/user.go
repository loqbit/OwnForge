package dto

type RegisterRequest struct {
	Phone    string `json:"phone" binding:"required,min=6,max=20"`
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=20,alphanum"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterResponse struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}
