package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	httpx "auction-system/server-go/internal/http"

	"github.com/go-sql-driver/mysql"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > 32 {
		return LoginResponse{}, httpx.InvalidParam("昵称为空或过长")
	}

	if user, ok, err := s.repo.FindByNickname(ctx, nickname); err != nil {
		return LoginResponse{}, err
	} else if ok {
		return LoginResponse{Token: user.Token, User: withoutToken(user)}, nil
	}

	user, err := s.repo.CreateMockUser(ctx, nickname, req.Avatar, mockKind(nickname))
	if err != nil {
		if isDuplicateEntry(err) {
			user, ok, findErr := s.repo.FindByNickname(ctx, nickname)
			if findErr != nil {
				return LoginResponse{}, findErr
			}
			if ok {
				return LoginResponse{Token: user.Token, User: withoutToken(user)}, nil
			}
		}
		return LoginResponse{}, err
	}
	return LoginResponse{Token: user.Token, User: withoutToken(user)}, nil
}

func MockToken(kind string, userID int64) string {
	return fmt.Sprintf("mock-token-%s-%03d", kind, userID)
}

func mockKind(nickname string) string {
	for _, prefix := range []string{"主播", "商家", "卖家"} {
		if strings.HasPrefix(nickname, prefix) {
			return "seller"
		}
	}
	return "user"
}

func withoutToken(user User) User {
	user.Token = ""
	return user
}

func isDuplicateEntry(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
