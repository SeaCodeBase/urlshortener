package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"go.uber.org/zap"
)

var (
	ErrPasskeyNotOwned   = errors.New("passkey does not belong to user")
	ErrLastPasskey       = errors.New("cannot delete last passkey")
	ErrInvalidSession    = errors.New("invalid session data")
	ErrCredentialInvalid = errors.New("credential verification failed")
)

type passkeyService struct {
	passkeyRepo repository.PasskeyRepository
	userRepo    repository.UserRepository
	webauthn    *webauthn.WebAuthn
}

func NewPasskeyService(passkeyRepo repository.PasskeyRepository, userRepo repository.UserRepository, rpID, rpOrigin, rpName string) (PasskeyService, error) {
	wconfig := &webauthn.Config{
		RPDisplayName: rpName,
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	}
	w, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create webauthn: %w", err)
	}
	return &passkeyService{
		passkeyRepo: passkeyRepo,
		userRepo:    userRepo,
		webauthn:    w,
	}, nil
}

// webAuthnUser adapts our User model to webauthn.User interface
type webAuthnUser struct {
	id          uint64
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.id))
}

func (u *webAuthnUser) WebAuthnName() string {
	return u.name
}

func (u *webAuthnUser) WebAuthnDisplayName() string {
	return u.displayName
}

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.credentials
}

func (s *passkeyService) toWebAuthnUser(ctx context.Context, user *model.User) (*webAuthnUser, error) {
	passkeys, err := s.passkeyRepo.ListByUserID(ctx, user.ID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to list passkeys for user",
			zap.Uint64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, err
	}
	credentials := make([]webauthn.Credential, len(passkeys))
	for i, pk := range passkeys {
		credentials[i] = webauthn.Credential{
			ID:              pk.CredentialID,
			PublicKey:       pk.PublicKey,
			AttestationType: "",
			Authenticator: webauthn.Authenticator{
				SignCount: pk.Counter,
			},
		}
	}
	displayName := user.Email
	if user.DisplayName != nil && *user.DisplayName != "" {
		displayName = *user.DisplayName
	}
	return &webAuthnUser{
		id:          user.ID,
		name:        user.Email,
		displayName: displayName,
		credentials: credentials,
	}, nil
}

func (s *passkeyService) BeginRegistration(ctx context.Context, userID uint64) (*protocol.CredentialCreation, string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get user for registration",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	waUser, err := s.toWebAuthnUser(ctx, user)
	if err != nil {
		return nil, "", err
	}
	options, session, err := s.webauthn.BeginRegistration(waUser)
	if err != nil {
		logger.Error(ctx, "passkey-service: WebAuthn BeginRegistration failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to marshal session data",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	return options, base64.StdEncoding.EncodeToString(sessionBytes), nil
}

func (s *passkeyService) FinishRegistration(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte, name string) (*model.Passkey, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get user for finish registration",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	waUser, err := s.toWebAuthnUser(ctx, user)
	if err != nil {
		return nil, err
	}
	sessionBytes, err := base64.StdEncoding.DecodeString(sessionData)
	if err != nil {
		return nil, ErrInvalidSession
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return nil, ErrInvalidSession
	}
	// Parse the credential creation response from JSON
	parsedResponse, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(credentialJSON))
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to parse credential creation response",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", ErrCredentialInvalid, err)
	}
	credential, err := s.webauthn.CreateCredential(waUser, session, parsedResponse)
	if err != nil {
		logger.Error(ctx, "passkey-service: WebAuthn CreateCredential failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("%w: %v", ErrCredentialInvalid, err)
	}
	passkey := &model.Passkey{
		UserID:       userID,
		Name:         name,
		CredentialID: credential.ID,
		PublicKey:    credential.PublicKey,
		Counter:      credential.Authenticator.SignCount,
	}
	if err := s.passkeyRepo.Create(ctx, passkey); err != nil {
		logger.Error(ctx, "passkey-service: failed to store credential",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return passkey, nil
}

func (s *passkeyService) BeginLogin(ctx context.Context, userID uint64) (*protocol.CredentialAssertion, string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get user for login",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	waUser, err := s.toWebAuthnUser(ctx, user)
	if err != nil {
		return nil, "", err
	}
	options, session, err := s.webauthn.BeginLogin(waUser)
	if err != nil {
		logger.Error(ctx, "passkey-service: WebAuthn BeginLogin failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	sessionBytes, err := json.Marshal(session)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to marshal login session data",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, "", err
	}
	return options, base64.StdEncoding.EncodeToString(sessionBytes), nil
}

func (s *passkeyService) FinishLogin(ctx context.Context, userID uint64, sessionData string, credentialJSON []byte) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get user for finish login",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	waUser, err := s.toWebAuthnUser(ctx, user)
	if err != nil {
		return err
	}
	sessionBytes, err := base64.StdEncoding.DecodeString(sessionData)
	if err != nil {
		return ErrInvalidSession
	}
	var session webauthn.SessionData
	if err := json.Unmarshal(sessionBytes, &session); err != nil {
		return ErrInvalidSession
	}
	// Parse the credential assertion response from JSON
	parsedResponse, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(credentialJSON))
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to parse credential assertion response",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrCredentialInvalid, err)
	}
	credential, err := s.webauthn.ValidateLogin(waUser, session, parsedResponse)
	if err != nil {
		logger.Error(ctx, "passkey-service: WebAuthn ValidateLogin failed",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("%w: %v", ErrCredentialInvalid, err)
	}
	// Find and update the passkey counter
	passkey, err := s.passkeyRepo.GetByCredentialID(ctx, credential.ID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to get passkey by credential ID",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	if err := s.passkeyRepo.UpdateCounter(ctx, passkey.ID, credential.Authenticator.SignCount); err != nil {
		logger.Error(ctx, "passkey-service: failed to update counter",
			zap.Uint64("passkey_id", passkey.ID),
			zap.Error(err),
		)
		return err
	}
	if err := s.passkeyRepo.UpdateLastUsedAt(ctx, passkey.ID); err != nil {
		logger.Error(ctx, "passkey-service: failed to update last used at",
			zap.Uint64("passkey_id", passkey.ID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *passkeyService) List(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	passkeys, err := s.passkeyRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to list passkeys",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return passkeys, nil
}

func (s *passkeyService) Rename(ctx context.Context, userID, passkeyID uint64, name string) error {
	passkeys, err := s.passkeyRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to list passkeys for rename",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	found := false
	for _, pk := range passkeys {
		if pk.ID == passkeyID {
			found = true
			break
		}
	}
	if !found {
		return ErrPasskeyNotOwned
	}
	if err := s.passkeyRepo.UpdateName(ctx, passkeyID, name); err != nil {
		logger.Error(ctx, "passkey-service: failed to rename passkey",
			zap.Uint64("passkey_id", passkeyID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *passkeyService) Delete(ctx context.Context, userID, passkeyID uint64) error {
	passkeys, err := s.passkeyRepo.ListByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to list passkeys for delete",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return err
	}
	found := false
	for _, pk := range passkeys {
		if pk.ID == passkeyID {
			found = true
			break
		}
	}
	if !found {
		return ErrPasskeyNotOwned
	}
	if len(passkeys) == 1 {
		return ErrLastPasskey
	}
	if err := s.passkeyRepo.Delete(ctx, passkeyID); err != nil {
		logger.Error(ctx, "passkey-service: failed to delete passkey",
			zap.Uint64("passkey_id", passkeyID),
			zap.Error(err),
		)
		return err
	}
	return nil
}

func (s *passkeyService) HasPasskeys(ctx context.Context, userID uint64) (bool, error) {
	count, err := s.passkeyRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.Error(ctx, "passkey-service: failed to count passkeys",
			zap.Uint64("user_id", userID),
			zap.Error(err),
		)
		return false, err
	}
	return count > 0, nil
}
