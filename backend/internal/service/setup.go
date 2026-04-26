package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/jonathanhu237/rota/backend/internal/email"
	"github.com/jonathanhu237/rota/backend/internal/model"
	"github.com/jonathanhu237/rota/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const setupTokenSizeBytes = 32

type setupUserRepository = repository.SetupUserRepository
type setupTokenRepository = repository.SetupTokenRepositoryWriter
type setupTxManager = repository.SetupTxRunner

type setupOutboxRepository interface {
	EnqueueTx(ctx context.Context, tx *sql.Tx, msg email.Message, opts ...repository.OutboxOption) error
}

type setupLogger interface {
	Error(msg string, args ...any)
	Warn(msg string, args ...any)
}

type SetupFlowConfig struct {
	TxManager             setupTxManager
	SetupTokenRepo        setupTokenRepository
	OutboxRepo            setupOutboxRepository
	Logger                setupLogger
	AppBaseURL            string
	InvitationTokenTTL    time.Duration
	PasswordResetTokenTTL time.Duration
	Clock                 func() time.Time
	RandomReader          io.Reader
}

type SetupTokenPreview struct {
	Email   string
	Name    string
	Purpose model.SetupTokenPurpose
}

type SetupPasswordInput struct {
	Token    string
	Password string
}

type setupFlowHelper struct {
	txManager             setupTxManager
	setupTokenRepo        setupTokenRepository
	outboxRepo            setupOutboxRepository
	logger                setupLogger
	appBaseURL            string
	invitationTokenTTL    time.Duration
	passwordResetTokenTTL time.Duration
	clock                 func() time.Time
	randomReader          io.Reader
}

func newSetupFlowHelper(config SetupFlowConfig) *setupFlowHelper {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	clock := config.Clock
	if clock == nil {
		clock = time.Now
	}

	randomReader := config.RandomReader
	if randomReader == nil {
		randomReader = rand.Reader
	}

	return &setupFlowHelper{
		txManager:             config.TxManager,
		setupTokenRepo:        config.SetupTokenRepo,
		outboxRepo:            config.OutboxRepo,
		logger:                logger,
		appBaseURL:            strings.TrimSpace(config.AppBaseURL),
		invitationTokenTTL:    config.InvitationTokenTTL,
		passwordResetTokenTTL: config.PasswordResetTokenTTL,
		clock:                 clock,
		randomReader:          randomReader,
	}
}

func (h *setupFlowHelper) issueToken(
	ctx context.Context,
	tokenRepo setupTokenRepository,
	userID int64,
	purpose model.SetupTokenPurpose,
	ttl time.Duration,
) (string, error) {
	rawToken, tokenHash, err := generateSetupToken(h.randomReader)
	if err != nil {
		return "", err
	}

	now := h.clock()
	if err := tokenRepo.InvalidateUnusedTokens(ctx, userID, purpose, now); err != nil {
		return "", err
	}

	_, err = tokenRepo.Create(ctx, repository.CreateSetupTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		Purpose:   purpose,
		ExpiresAt: now.Add(ttl),
	})
	if err != nil {
		return "", err
	}

	return rawToken, nil
}

func (h *setupFlowHelper) enqueueInvitationTx(
	ctx context.Context,
	tx *sql.Tx,
	user *model.User,
	rawToken string,
) error {
	return h.enqueueEmailTx(ctx, tx, email.BuildInvitationMessage(email.TemplateData{
		To:         user.Email,
		Name:       user.Name,
		BaseURL:    h.appBaseURL,
		Token:      rawToken,
		Language:   "en",
		Expiration: h.invitationTokenTTL,
	}), repository.WithOutboxUserID(user.ID))
}

func (h *setupFlowHelper) enqueuePasswordResetTx(
	ctx context.Context,
	tx *sql.Tx,
	user *model.User,
	rawToken string,
) error {
	return h.enqueueEmailTx(ctx, tx, email.BuildPasswordResetMessage(email.TemplateData{
		To:         user.Email,
		Name:       user.Name,
		BaseURL:    h.appBaseURL,
		Token:      rawToken,
		Language:   "en",
		Expiration: h.passwordResetTokenTTL,
	}), repository.WithOutboxUserID(user.ID))
}

func (h *setupFlowHelper) enqueueEmailTx(
	ctx context.Context,
	tx *sql.Tx,
	msg email.Message,
	opts ...repository.OutboxOption,
) error {
	if h.outboxRepo == nil {
		return nil
	}

	return h.outboxRepo.EnqueueTx(ctx, tx, msg, opts...)
}

func (h *setupFlowHelper) resolveToken(
	ctx context.Context,
	tokenRepo setupTokenRepository,
	rawToken string,
) (*model.SetupToken, string, error) {
	tokenHash, err := validateAndHashSetupToken(rawToken)
	if err != nil {
		return nil, "", err
	}

	token, err := tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, "", err
	}

	if token.UsedAt != nil {
		return nil, tokenHash, model.ErrTokenUsed
	}
	if !token.ExpiresAt.After(h.clock()) {
		return nil, tokenHash, model.ErrTokenExpired
	}

	return token, tokenHash, nil
}

func (h *setupFlowHelper) activatePassword(
	ctx context.Context,
	userRepo setupUserRepository,
	tokenRepo setupTokenRepository,
	input SetupPasswordInput,
) (*model.SetupToken, error) {
	if err := model.ValidatePassword(input.Password); err != nil {
		return nil, err
	}

	token, _, err := h.resolveToken(ctx, tokenRepo, input.Token)
	if err != nil {
		return nil, err
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := h.clock()
	if err := tokenRepo.MarkUsed(ctx, token.ID, now); err != nil {
		return nil, err
	}
	if err := tokenRepo.InvalidateAllUnusedTokens(ctx, token.UserID, now); err != nil {
		return nil, err
	}
	if _, err := userRepo.SetPasswordAndStatus(ctx, repository.SetUserPasswordParams{
		ID:           token.UserID,
		PasswordHash: string(passwordHash),
		Status:       model.UserStatusActive,
	}); err != nil {
		return nil, mapRepositoryError(err)
	}

	return token, nil
}

func generateSetupToken(randomReader io.Reader) (string, string, error) {
	rawBytes := make([]byte, setupTokenSizeBytes)
	if _, err := io.ReadFull(randomReader, rawBytes); err != nil {
		return "", "", err
	}

	rawToken := base64.RawURLEncoding.EncodeToString(rawBytes)
	return rawToken, hashSetupToken(rawToken), nil
}

func validateAndHashSetupToken(rawToken string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(rawToken))
	if err != nil || len(decoded) != setupTokenSizeBytes {
		return "", model.ErrInvalidToken
	}

	return hashSetupToken(strings.TrimSpace(rawToken)), nil
}

func hashSetupToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

func mapSetupTokenError(err error) error {
	switch {
	case errors.Is(err, repository.ErrSetupTokenAlreadyExists):
		return err
	case errors.Is(err, model.ErrTokenNotFound):
		return model.ErrTokenNotFound
	default:
		return err
	}
}
