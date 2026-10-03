package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/alexedwards/scs/v2"
	"golang.org/x/crypto/bcrypt"
	"homedashboard/internal/domain"
	"homedashboard/internal/storage"
)

type Service struct {
	Store      *storage.Repository
	Sessions   *scs.SessionManager
	SetupToken string
	dummyHash  []byte
}

func New(store *storage.Repository, sessions *scs.SessionManager, setupToken string) (*Service, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte("invalid-account-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Service{Store: store, Sessions: sessions, SetupToken: setupToken, dummyHash: hash}, nil
}

func (s *Service) Setup(ctx context.Context, token, username, password string) (domain.User, error) {
	needsSetup, err := s.Store.NeedsSetup(ctx)
	if err != nil {
		return domain.User{}, err
	}
	if !needsSetup {
		return domain.User{}, domain.ErrAlreadySetup
	}
	want, got := sha256.Sum256([]byte(s.SetupToken)), sha256.Sum256([]byte(token))
	if s.SetupToken == "" || subtle.ConstantTimeCompare(want[:], got[:]) != 1 {
		return domain.User{}, domain.ErrUnauthorized
	}
	username = strings.TrimSpace(username)
	if utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 64 || strings.ContainsAny(username, "\r\n\t ") {
		return domain.User{}, &domain.ValidationError{Message: "아이디는 공백 없이 3~64자로 입력해주세요."}
	}
	if utf8.RuneCountInString(password) < 12 || len(password) > 72 {
		return domain.User{}, &domain.ValidationError{Message: "비밀번호는 12자 이상, UTF-8 기준 72바이트 이하로 입력해주세요."}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, err
	}
	return s.Store.Setup(ctx, username, string(hash))
}

func (s *Service) Login(ctx context.Context, username, password string) (domain.User, error) {
	user, err := s.Store.FindUser(ctx, "username", strings.TrimSpace(username))
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, err
		}
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return domain.User{}, domain.ErrUnauthorized
	}
	if len(password) > 72 || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	return domain.User{ID: user.ID, Username: user.Username}, nil
}

func (s *Service) StartSession(ctx context.Context, user domain.User) error {
	if err := s.Sessions.RenewToken(ctx); err != nil {
		return err
	}
	s.Sessions.Put(ctx, "userID", user.ID)
	return nil
}

func (s *Service) CurrentUser(ctx context.Context) (domain.User, error) {
	id := s.Sessions.GetString(ctx, "userID")
	if id == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	user, err := s.Store.FindUser(ctx, "id", id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, domain.ErrUnauthorized
	}
	return domain.User{ID: user.ID, Username: user.Username}, err
}
