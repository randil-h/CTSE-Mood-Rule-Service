package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/eventbus"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/grpc/clients"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"go.uber.org/zap"
)

type MoodUpdateRequest struct {
	UserID string `json:"userId"`
	Mood   string `json:"mood"`
}

type MoodUpdateResponse struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

type MoodHandler struct {
	authServiceURL string
	bus            *eventbus.EventBus
	productClient  *clients.ProductCatalogClient
}

func NewMoodHandler(authServiceURL string, bus *eventbus.EventBus, productClient *clients.ProductCatalogClient) *MoodHandler {
	return &MoodHandler{
		authServiceURL: authServiceURL,
		bus:            bus,
		productClient:  productClient,
	}
}

func (h *MoodHandler) UpdateMood(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req MoodUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error(ctx, "Failed to decode mood update request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	logger.Info(ctx, "Received mood update request",
		zap.String("user_id", req.UserID),
		zap.String("mood", req.Mood))

	// Extract auth token from Authorization header
	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		logger.Warn(ctx, "Missing authorization token")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Forward mood update to Auth Service
	if err := h.forwardToAuthService(ctx, req.Mood, authToken); err != nil {
		logger.Error(ctx, "Failed to update mood in Auth Service", zap.Error(err))
		http.Error(w, "Failed to update mood", http.StatusInternalServerError)
		return
	}

	// Publish mood change event
	h.publishMoodChangeEvent(ctx, req.UserID, req.Mood)

	// Notify Product Catalog Service via gRPC
	if h.productClient != nil {
		if err := h.notifyProductCatalog(ctx, req.UserID, req.Mood); err != nil {
			logger.Warn(ctx, "Failed to notify Product Catalog Service",
				zap.String("user_id", req.UserID),
				zap.Error(err))
			// Don't fail the request if notification fails
		}
	}

	// Return success response
	resp := MoodUpdateResponse{
		Message: "Mood updated successfully",
		Success: true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)

	logger.Info(ctx, "Mood update completed successfully",
		zap.String("user_id", req.UserID),
		zap.String("mood", req.Mood))
}

func (h *MoodHandler) forwardToAuthService(ctx context.Context, mood string, authToken string) error {
	// Prepare request payload
	payload := map[string]string{"mood": mood}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal mood payload: %w", err)
	}

	// Create HTTP request to Auth Service
	url := fmt.Sprintf("%s/auth/mood", h.authServiceURL)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authToken)

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Auth Service: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Auth Service returned error status %d: %s", resp.StatusCode, string(body))
	}

	logger.Info(ctx, "Successfully updated mood in Auth Service")
	return nil
}

func (h *MoodHandler) publishMoodChangeEvent(ctx context.Context, userID string, mood string) {
	if h.bus == nil {
		return
	}

	h.bus.Publish(ctx, eventbus.EventTypeMoodChanged, map[string]interface{}{
		"event_type": "mood_changed",
		"user_id":    userID,
		"mood":       mood,
		"timestamp":  time.Now().UTC(),
	})

	logger.Info(ctx, "Published mood change event",
		zap.String("user_id", userID),
		zap.String("mood", mood))
}

func (h *MoodHandler) notifyProductCatalog(ctx context.Context, userID string, mood string) error {
	// Generate session ID for tracking
	sessionID := fmt.Sprintf("sess_%d", time.Now().Unix())
	traceID := fmt.Sprintf("trace_%s_%d", userID, time.Now().UnixNano())

	// Call gRPC method to notify Product Catalog Service
	ack, err := h.productClient.NotifyMoodUpdate(
		ctx,
		userID,
		mood,
		"",
		sessionID,
		traceID,
	)

	if err != nil {
		return fmt.Errorf("failed to notify product catalog: %w", err)
	}

	logger.Info(ctx, "Product Catalog Service notified of mood update",
		zap.String("user_id", userID),
		zap.String("mood", mood),
		zap.String("correlation_id", ack.CorrelationId),
		zap.Int32("recommendations_generated", ack.RecommendationsGenerated))

	return nil
}
