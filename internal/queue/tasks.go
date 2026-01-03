package queue

import (
	"encoding/json"

	"github.com/hibiken/asynq"
)

// Task type constants
const (
	TypeSendEmail        = "email:send"
	TypeSendOTP          = "email:otp"
	TypeSendWelcome      = "email:welcome"
	TypeSendPasswordReset = "email:password_reset"
	TypeNotification     = "notification:send"
	TypeProductIndexing  = "product:index"
)

// ============================================
// Email Tasks
// ============================================

type SendEmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func NewSendEmailTask(to, subject, body string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendEmailPayload{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendEmail, payload), nil
}

type SendOTPPayload struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
	Name  string `json:"name"`
}

func NewSendOTPTask(email, otp, name string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendOTPPayload{
		Email: email,
		OTP:   otp,
		Name:  name,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendOTP, payload), nil
}

type SendWelcomePayload struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func NewSendWelcomeTask(email, name string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendWelcomePayload{
		Email: email,
		Name:  name,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendWelcome, payload), nil
}

type SendPasswordResetPayload struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	ResetLink string `json:"reset_link"`
}

func NewSendPasswordResetTask(email, name, resetLink string) (*asynq.Task, error) {
	payload, err := json.Marshal(SendPasswordResetPayload{
		Email:     email,
		Name:      name,
		ResetLink: resetLink,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendPasswordReset, payload), nil
}

// ============================================
// Notification Tasks
// ============================================

type NotificationPayload struct {
	UserID  int    `json:"user_id"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"` // info, warning, success, error
}

func NewNotificationTask(userID int, title, message, notifType string) (*asynq.Task, error) {
	payload, err := json.Marshal(NotificationPayload{
		UserID:  userID,
		Title:   title,
		Message: message,
		Type:    notifType,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeNotification, payload), nil
}

// ============================================
// Product Tasks
// ============================================

type ProductIndexPayload struct {
	ProductID   int    `json:"product_id"`
	ProductUUID string `json:"product_uuid"`
	Action      string `json:"action"` // create, update, delete
}

func NewProductIndexTask(productID int, productUUID, action string) (*asynq.Task, error) {
	payload, err := json.Marshal(ProductIndexPayload{
		ProductID:   productID,
		ProductUUID: productUUID,
		Action:      action,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeProductIndexing, payload), nil
}
