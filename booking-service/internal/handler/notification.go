package handler

import (
	"context"
	"fmt"
	"log"
	"strconv"
)

func (h *Handler) notifyCustomer(ctx context.Context, bookingID int, notificationType string, title string, message string) {
	h.notifyBookingParticipant(ctx, bookingID, true, notificationType, title, message)
}

func (h *Handler) notifyWorker(ctx context.Context, bookingID int, notificationType string, title string, message string) {
	h.notifyBookingParticipant(ctx, bookingID, false, notificationType, title, message)
}

func (h *Handler) notifyWorkerChatAction(ctx context.Context, bookingID int, notificationType string, title string, message string) {
	h.notifyBookingParticipantAction(ctx, bookingID, false, notificationType, title, message, "booking_chat", strconv.Itoa(bookingID), "Open chat")
}

func (h *Handler) notifyBookingParticipant(ctx context.Context, bookingID int, customer bool, notificationType string, title string, message string) {
	h.notifyBookingParticipantAction(ctx, bookingID, customer, notificationType, title, message, "", "", "")
}

func (h *Handler) notifyBookingParticipantAction(ctx context.Context, bookingID int, customer bool, notificationType string, title string, message string, actionType string, actionRef string, actionLabel string) {
	if h.notificationClient == nil || bookingID <= 0 {
		return
	}

	users, err := h.bookingService.BookingUsers(ctx, bookingID)
	if err != nil {
		log.Printf("booking notification skipped for booking %d: %v", bookingID, err)
		return
	}

	userID := users.WorkerUserID
	if customer {
		userID = users.CustomerUserID
	}
	if title == "" {
		title = fmt.Sprintf("Booking #%d", bookingID)
	}
	h.notificationClient.CreateAction(ctx, userID, notificationType, title, message, actionType, actionRef, actionLabel)
}
