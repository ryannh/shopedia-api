package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskHandler struct {
	DB *pgxpool.Pool
}

func NewTaskHandler(db *pgxpool.Pool) *TaskHandler {
	return &TaskHandler{DB: db}
}

// ============================================
// Email Handlers
// ============================================

func (h *TaskHandler) HandleSendEmail(ctx context.Context, t *asynq.Task) error {
	var payload SendEmailPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[Email] Sending to: %s, Subject: %s", payload.To, payload.Subject)

	err := sendEmail(payload.To, payload.Subject, payload.Body)
	if err != nil {
		log.Printf("[Email] Failed to send: %v", err)
		return err
	}

	log.Printf("[Email] Successfully sent to: %s", payload.To)
	return nil
}

func (h *TaskHandler) HandleSendOTP(ctx context.Context, t *asynq.Task) error {
	var payload SendOTPPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[OTP] Sending OTP to: %s", payload.Email)

	subject := "Kode Verifikasi OTP - Shopedia"
	body := fmt.Sprintf(`
Halo %s,

Kode OTP Anda adalah: %s

Kode ini berlaku selama 5 menit.
Jangan bagikan kode ini kepada siapapun.

Terima kasih,
Tim Shopedia
`, payload.Name, payload.OTP)

	err := sendEmail(payload.Email, subject, body)
	if err != nil {
		log.Printf("[OTP] Failed to send: %v", err)
		return err
	}

	log.Printf("[OTP] Successfully sent to: %s", payload.Email)
	return nil
}

func (h *TaskHandler) HandleSendWelcome(ctx context.Context, t *asynq.Task) error {
	var payload SendWelcomePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[Welcome] Sending welcome email to: %s", payload.Email)

	subject := "Selamat Datang di Shopedia!"
	body := fmt.Sprintf(`
Halo %s,

Selamat datang di Shopedia! Akun Anda telah berhasil dibuat.

Anda sekarang dapat:
- Menjelajahi ribuan produk
- Membuat toko dan mulai berjualan
- Mendapatkan penawaran eksklusif

Terima kasih telah bergabung!

Salam,
Tim Shopedia
`, payload.Name)

	err := sendEmail(payload.Email, subject, body)
	if err != nil {
		log.Printf("[Welcome] Failed to send: %v", err)
		return err
	}

	log.Printf("[Welcome] Successfully sent to: %s", payload.Email)
	return nil
}

func (h *TaskHandler) HandleSendPasswordReset(ctx context.Context, t *asynq.Task) error {
	var payload SendPasswordResetPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[PasswordReset] Sending reset email to: %s", payload.Email)

	subject := "Reset Password - Shopedia"
	body := fmt.Sprintf(`
Halo %s,

Kami menerima permintaan untuk reset password akun Anda.

Klik link berikut untuk reset password:
%s

Link ini berlaku selama 1 jam.
Jika Anda tidak meminta reset password, abaikan email ini.

Salam,
Tim Shopedia
`, payload.Name, payload.ResetLink)

	err := sendEmail(payload.Email, subject, body)
	if err != nil {
		log.Printf("[PasswordReset] Failed to send: %v", err)
		return err
	}

	log.Printf("[PasswordReset] Successfully sent to: %s", payload.Email)
	return nil
}

// ============================================
// Notification Handler
// ============================================

func (h *TaskHandler) HandleNotification(ctx context.Context, t *asynq.Task) error {
	var payload NotificationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[Notification] User: %d, Title: %s, Type: %s", payload.UserID, payload.Title, payload.Type)

	// TODO: Implement actual notification logic (push notification, websocket, etc.)
	// For now, just log it
	// Example: Save to notifications table
	/*
		_, err := h.DB.Exec(ctx, `
			INSERT INTO notifications (user_id, title, message, type, is_read, created_at)
			VALUES ($1, $2, $3, $4, FALSE, NOW())`,
			payload.UserID, payload.Title, payload.Message, payload.Type)
		if err != nil {
			return err
		}
	*/

	log.Printf("[Notification] Successfully processed for user: %d", payload.UserID)
	return nil
}

// ============================================
// Product Handler
// ============================================

func (h *TaskHandler) HandleProductIndexing(ctx context.Context, t *asynq.Task) error {
	var payload ProductIndexPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	log.Printf("[ProductIndex] Action: %s, ProductID: %d, UUID: %s", payload.Action, payload.ProductID, payload.ProductUUID)

	// TODO: Implement search indexing logic (Elasticsearch, Meilisearch, etc.)
	// For now, just log it

	switch payload.Action {
	case "create":
		log.Printf("[ProductIndex] Indexing new product: %s", payload.ProductUUID)
	case "update":
		log.Printf("[ProductIndex] Updating product index: %s", payload.ProductUUID)
	case "delete":
		log.Printf("[ProductIndex] Removing product from index: %s", payload.ProductUUID)
	}

	log.Printf("[ProductIndex] Successfully processed: %s", payload.ProductUUID)
	return nil
}

// ============================================
// Helper Functions
// ============================================

func sendEmail(to, subject, body string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	if smtpHost == "" || smtpUser == "" {
		log.Printf("[Email] SMTP not configured, skipping email send")
		return nil
	}

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)

	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		to, subject, body))

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	return smtp.SendMail(addr, auth, smtpUser, []string{to}, msg)
}
